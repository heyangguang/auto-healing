package healing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// Execute 执行流程
func (e *FlowExecutor) Execute(ctx context.Context, instance *model.FlowInstance) (err error) {
	runCtx, cancel := newFlowRunContext(ctx, instance.ID)
	started := false
	defer func() {
		if err != nil {
			e.handleExecuteError(detachContext(runCtx), instance, started, err)
		}
	}()
	defer cleanupFlowRun(instance.ID, cancel)

	logger.Exec("FLOW").Info("[%s] 开始执行流程实例", instance.ID.String()[:8])
	if err = e.startFlowInstance(runCtx, instance); err != nil {
		return err
	}
	started = true

	nodes, edges, err := e.parseFlowSnapshot(instance)
	if err != nil {
		e.fail(runCtx, instance, "解析流程定义失败: "+err.Error())
		return err
	}

	startNode := e.findStartNode(nodes)
	if startNode == nil {
		e.fail(runCtx, instance, "找不到起始节点")
		return nil
	}
	return e.executeNode(runCtx, instance, nodes, edges, startNode)
}

func newFlowRunContext(ctx context.Context, instanceID uuid.UUID) (context.Context, context.CancelFunc) {
	runCtx, cancel := context.WithCancel(ctx)
	runningFlowCancels.Store(instanceID, cancel)
	return runCtx, cancel
}

func cleanupFlowRun(instanceID uuid.UUID, cancel context.CancelFunc) {
	runningFlowCancels.Delete(instanceID)
	cancel()
}

func (e *FlowExecutor) handleExecuteError(ctx context.Context, instance *model.FlowInstance, started bool, execErr error) {
	if shouldIgnoreExecuteError(execErr) {
		return
	}
	status, err := e.flowInstanceStatus(ctx, instance.ID)
	if err != nil {
		logger.Exec("FLOW").Error("[%s] 查询流程实例状态失败: %v", shortID(instance), err)
		return
	}
	if !shouldFailExecuteError(status, started) {
		return
	}
	e.fail(ctx, instance, formatExecuteError(started, execErr))
}

func shouldIgnoreExecuteError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func shouldFailExecuteError(status string, started bool) bool {
	switch status {
	case model.FlowInstanceStatusCompleted, model.FlowInstanceStatusFailed, model.FlowInstanceStatusCancelled:
		return false
	}
	if started {
		return status == model.FlowInstanceStatusPending ||
			status == model.FlowInstanceStatusRunning ||
			status == model.FlowInstanceStatusWaitingApproval
	}
	return status == model.FlowInstanceStatusPending
}

func formatExecuteError(started bool, err error) string {
	if started {
		return "流程执行异常: " + err.Error()
	}
	return "启动流程实例失败: " + err.Error()
}

func (e *FlowExecutor) flowInstanceStatus(ctx context.Context, instanceID uuid.UUID) (string, error) {
	instance, err := e.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return "", err
	}
	return instance.Status, nil
}

func (e *FlowExecutor) startFlowInstance(ctx context.Context, instance *model.FlowInstance) error {
	started, err := e.instanceRepo.Start(ctx, instance.ID)
	if err != nil {
		return err
	}
	if !started {
		return fmt.Errorf("流程实例状态不允许启动")
	}
	startedAt := time.Now()
	instance.StartedAt = &startedAt
	instance.Status = model.FlowInstanceStatusRunning
	return nil
}

// RetryFromNode 从指定节点重试执行流程实例
func (e *FlowExecutor) RetryFromNode(ctx context.Context, instance *model.FlowInstance, fromNodeID string) error {
	logger.Exec("FLOW").Info("[%s] 从节点 %s 重试执行流程实例", instance.ID.String()[:8], fromNodeID)
	if err := e.restartFailedInstance(ctx, instance); err != nil {
		return err
	}

	runCtx, cancel := newFlowRunContext(ctx, instance.ID)
	defer cleanupFlowRun(instance.ID, cancel)

	nodes, edges, err := e.parseFlowSnapshot(instance)
	if err != nil {
		e.fail(runCtx, instance, "解析流程定义失败: "+err.Error())
		return err
	}

	targetNode, err := resolveRetryTargetNode(nodes, instance, fromNodeID)
	if err != nil {
		e.fail(runCtx, instance, err.Error())
		return err
	}
	return e.executeNode(runCtx, instance, nodes, edges, targetNode)
}

func (e *FlowExecutor) restartFailedInstance(ctx context.Context, instance *model.FlowInstance) error {
	updated, err := e.instanceRepo.UpdateStatusWithIncidentSync(
		ctx,
		instance.ID,
		[]string{model.FlowInstanceStatusFailed},
		model.FlowInstanceStatusRunning,
		"",
		instanceIncidentSyncOptions(instance, "processing"),
	)
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("只能重试失败的流程实例，当前状态已变更")
	}
	instance.Status = model.FlowInstanceStatusRunning
	instance.ErrorMessage = ""
	return nil
}

func resolveRetryTargetNode(nodes []model.FlowNode, instance *model.FlowInstance, fromNodeID string) (*model.FlowNode, error) {
	targetID := fromNodeID
	if targetID == "" {
		targetID = instance.CurrentNodeID
	}
	for i := range nodes {
		if nodes[i].ID == targetID {
			return &nodes[i], nil
		}
	}
	if fromNodeID == "" {
		return nil, fmt.Errorf("找不到当前节点: %s", instance.CurrentNodeID)
	}
	return nil, fmt.Errorf("找不到节点: %s", fromNodeID)
}
