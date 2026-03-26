package healing

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

func (e *FlowExecutor) executeNode(ctx context.Context, instance *model.FlowInstance, nodes []model.FlowNode, edges []model.FlowEdge, node *model.FlowNode) error {
	if e.shouldStopFlowExecution(ctx, instance) {
		return nil
	}

	logger.Exec("NODE").Info("[%s] 执行节点 %s (%s)", shortID(instance), node.ID, node.Type)
	if err := e.persistInstance(ctx, setCurrentNode(instance, node.ID), "更新当前节点"); err != nil {
		return err
	}
	e.eventBus.PublishNodeStart(instance.ID, node.ID, node.Type, resolveNodeName(node))

	if nodeManagesOwnState(node.Type) {
		return e.executeManagedBranchNode(ctx, instance, nodes, edges, node)
	}
	return e.executeRegularNode(ctx, instance, nodes, edges, node)
}

func (e *FlowExecutor) shouldStopFlowExecution(ctx context.Context, instance *model.FlowInstance) bool {
	if err := ctx.Err(); err != nil {
		logger.Exec("FLOW").Warn("[%s] 流程上下文已取消，停止后续节点执行", shortID(instance))
		return true
	}
	if latest, err := e.instanceRepo.GetByID(ctx, instance.ID); err == nil && latest.Status == model.FlowInstanceStatusCancelled {
		logger.Exec("FLOW").Warn("[%s] 流程实例已取消，停止后续节点执行", shortID(instance))
		return true
	}
	return false
}

func setCurrentNode(instance *model.FlowInstance, nodeID string) *model.FlowInstance {
	instance.CurrentNodeID = nodeID
	return instance
}

func resolveNodeName(node *model.FlowNode) string {
	if node.Name != "" {
		return node.Name
	}
	if node.Config != nil {
		if label, ok := node.Config["label"].(string); ok {
			return label
		}
	}
	return ""
}

func nodeManagesOwnState(nodeType string) bool {
	return nodeType == model.NodeTypeApproval || nodeType == model.NodeTypeExecution || nodeType == model.NodeTypeCondition
}

func (e *FlowExecutor) executeManagedBranchNode(ctx context.Context, instance *model.FlowInstance, nodes []model.FlowNode, edges []model.FlowEdge, node *model.FlowNode) error {
	nodeCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	switch node.Type {
	case model.NodeTypeApproval:
		return e.executeApproval(nodeCtx, instance, node)
	case model.NodeTypeExecution:
		return e.executeExecutionWithBranch(nodeCtx, instance, nodes, edges, node)
	case model.NodeTypeCondition:
		return e.executeCondition(nodeCtx, instance, nodes, edges, node)
	default:
		return nil
	}
}

func (e *FlowExecutor) executeRegularNode(ctx context.Context, instance *model.FlowInstance, nodes []model.FlowNode, edges []model.FlowEdge, node *model.FlowNode) error {
	if err := e.setNodeState(ctx, instance, node.ID, "running", ""); err != nil {
		return err
	}
	nodeCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	err := e.runRegularNode(nodeCtx, instance, node)
	if err != nil {
		if stateErr := e.setNodeState(nodeCtx, instance, node.ID, "failed", err.Error()); stateErr != nil {
			return stateErr
		}
		e.eventBus.PublishNodeComplete(instance.ID, node.ID, node.Type, model.NodeStatusFailed, nil, nil, nil, "")
		e.fail(nodeCtx, instance, "节点执行失败: "+err.Error())
		return err
	}
	if node.Type == model.NodeTypeEnd {
		return nil
	}

	if err := e.setNodeState(nodeCtx, instance, node.ID, "completed", ""); err != nil {
		return err
	}
	e.eventBus.PublishNodeComplete(instance.ID, node.ID, node.Type, model.NodeStatusSuccess, nil, nil, nil, "default")
	if nextNode := e.findNextNode(nodes, edges, node.ID); nextNode != nil {
		return e.executeNode(ctx, instance, nodes, edges, nextNode)
	}
	return e.complete(ctx, instance)
}

func (e *FlowExecutor) runRegularNode(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	switch node.Type {
	case model.NodeTypeStart:
		e.logStartNode(ctx, instance, node)
		return nil
	case model.NodeTypeEnd:
		if err := e.logEndNode(ctx, instance, node); err != nil {
			return err
		}
		return e.complete(ctx, instance)
	case model.NodeTypeHostExtractor:
		return e.executeHostExtractor(ctx, instance, node)
	case model.NodeTypeCMDBValidator:
		return e.executeCMDBValidator(ctx, instance, node)
	case model.NodeTypeNotification:
		return e.executeNotification(ctx, instance, node)
	case model.NodeTypeSetVariable:
		return e.executeSetVariable(ctx, instance, node)
	case model.NodeTypeCompute:
		return e.executeCompute(ctx, instance, node)
	default:
		logger.Exec("NODE").Warn("未知节点类型: %s", node.Type)
		return nil
	}
}

func (e *FlowExecutor) logStartNode(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) {
	logDetails := map[string]interface{}{
		"input":   instance.Context,
		"process": []string{"初始化流程上下文"},
	}
	if instance.Context != nil {
		if incident, ok := instance.Context["incident"]; ok {
			logDetails["output"] = map[string]interface{}{"incident": incident}
			logDetails["process"] = []string{"初始化流程上下文", "输出 incident 到下游"}
		}
	}
	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "流程开始", logDetails)
}

func (e *FlowExecutor) logEndNode(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "流程结束", map[string]interface{}{
		"input":   instance.Context,
		"process": []string{"流程执行完毕"},
		"output":  nil,
	})
	if err := e.setNodeState(ctx, instance, node.ID, "completed", ""); err != nil {
		return err
	}
	e.eventBus.PublishNodeComplete(instance.ID, node.ID, node.Type, model.NodeStatusSuccess, nil, nil, nil, "")
	return nil
}
