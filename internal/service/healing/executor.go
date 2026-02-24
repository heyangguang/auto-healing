package healing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	cfg "github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/engine/provider/ansible"
	"github.com/company/auto-healing/internal/model"
	notificationSvc "github.com/company/auto-healing/internal/notification"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/service/execution"
	"github.com/google/uuid"
)

// FlowExecutor 流程执行器
type FlowExecutor struct {
	instanceRepo    *repository.FlowInstanceRepository
	approvalRepo    *repository.ApprovalTaskRepository
	flowRepo        *repository.HealingFlowRepository
	flowLogRepo     *repository.FlowLogRepository
	cmdbRepo        *repository.CMDBItemRepository
	gitRepoRepo     *repository.GitRepositoryRepository
	executionRepo   *repository.ExecutionRepository
	incidentRepo    *repository.IncidentRepository
	executionSvc    *execution.Service // 执行服务
	notificationSvc *notificationSvc.Service
	ansibleExecutor ansible.Executor
	eventBus        *EventBus // SSE 事件总线
}

// NewFlowExecutor 创建流程执行器
func NewFlowExecutor() *FlowExecutor {
	return &FlowExecutor{
		instanceRepo:    repository.NewFlowInstanceRepository(),
		approvalRepo:    repository.NewApprovalTaskRepository(),
		flowRepo:        repository.NewHealingFlowRepository(),
		flowLogRepo:     repository.NewFlowLogRepository(),
		cmdbRepo:        repository.NewCMDBItemRepository(),
		gitRepoRepo:     repository.NewGitRepositoryRepository(),
		executionRepo:   repository.NewExecutionRepository(),
		incidentRepo:    repository.NewIncidentRepository(),
		executionSvc:    execution.NewService(),
		notificationSvc: notificationSvc.NewService(database.DB, "Auto-Healing", "http://localhost:8080", "1.0.0"),
		ansibleExecutor: ansible.NewLocalExecutor(),
		eventBus:        GetEventBus(),
	}
}

// toFloat 将 interface{} 转换为 float64
func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

// shortID 返回实例ID的前8位，用于日志追踪
func shortID(instance *model.FlowInstance) string {
	return instance.ID.String()[:8]
}

// Execute 执行流程
func (e *FlowExecutor) Execute(ctx context.Context, instance *model.FlowInstance) error {
	logger.Exec("FLOW").Info("[%s] 开始执行流程实例", instance.ID.String()[:8])

	// 更新状态为运行中
	startedAt := time.Now()
	instance.StartedAt = &startedAt
	instance.Status = model.FlowInstanceStatusRunning
	e.instanceRepo.Update(ctx, instance)

	// 从实例快照解析节点和边
	nodes, edges, err := e.parseFlowSnapshot(instance)
	if err != nil {
		e.fail(ctx, instance, "解析流程定义失败: "+err.Error())
		return err
	}

	// 找到起始节点
	startNode := e.findStartNode(nodes)
	if startNode == nil {
		e.fail(ctx, instance, "找不到起始节点")
		return nil
	}

	// 从起始节点开始执行
	return e.executeNode(ctx, instance, nodes, edges, startNode)
}

// RetryFromNode 从指定节点重试执行流程实例
// 只能对 failed 状态的实例进行重试
func (e *FlowExecutor) RetryFromNode(ctx context.Context, instance *model.FlowInstance, fromNodeID string) error {
	logger.Exec("FLOW").Info("[%s] 从节点 %s 重试执行流程实例", instance.ID.String()[:8], fromNodeID)

	// 检查状态
	if instance.Status != model.FlowInstanceStatusFailed {
		return fmt.Errorf("只能重试失败的流程实例，当前状态: %s", instance.Status)
	}

	// 更新状态为运行中
	instance.Status = model.FlowInstanceStatusRunning
	instance.ErrorMessage = ""
	e.instanceRepo.Update(ctx, instance)

	// 从实例快照解析节点和边
	nodes, edges, err := e.parseFlowSnapshot(instance)
	if err != nil {
		e.fail(ctx, instance, "解析流程定义失败: "+err.Error())
		return err
	}

	// 找到指定节点
	var targetNode *model.FlowNode
	if fromNodeID == "" {
		// 如果没指定节点，从当前节点（失败节点）继续
		for i := range nodes {
			if nodes[i].ID == instance.CurrentNodeID {
				targetNode = &nodes[i]
				break
			}
		}
	} else {
		// 从指定节点开始
		for i := range nodes {
			if nodes[i].ID == fromNodeID {
				targetNode = &nodes[i]
				break
			}
		}
	}

	if targetNode == nil {
		errMsg := fmt.Sprintf("找不到节点: %s", fromNodeID)
		if fromNodeID == "" {
			errMsg = fmt.Sprintf("找不到当前节点: %s", instance.CurrentNodeID)
		}
		e.fail(ctx, instance, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 从目标节点开始执行
	return e.executeNode(ctx, instance, nodes, edges, targetNode)
}

// executeNode 执行节点

// setNodeState 更新节点状态到 instance.NodeStates 并持久化
// 自动计算 duration_ms：当从 running 转为终态（completed/failed/approved/rejected/partial）时
func (e *FlowExecutor) setNodeState(ctx context.Context, instance *model.FlowInstance, nodeID string, status string, errorMsg string) {
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	now := time.Now()
	state := map[string]interface{}{
		"status":     status,
		"updated_at": now.Format(time.RFC3339),
	}
	if status == "running" {
		state["started_at"] = now.Format(time.RFC3339)
	}
	if errorMsg != "" {
		state["error_message"] = errorMsg
	}
	// 合并已有状态（保留 started_at 等历史字段）
	if existing, ok := instance.NodeStates[nodeID].(map[string]interface{}); ok {
		for k, v := range state {
			existing[k] = v
		}
		// 计算耗时：从 started_at 到现在
		if status != "running" {
			if startedStr, ok := existing["started_at"].(string); ok {
				if startedAt, err := time.Parse(time.RFC3339, startedStr); err == nil {
					existing["duration_ms"] = now.Sub(startedAt).Milliseconds()
				}
			}
		}
		instance.NodeStates[nodeID] = existing
	} else {
		// 没有既存记录，直接设置（可能是跳过 running 直接到终态）
		if status != "running" {
			state["duration_ms"] = int64(0) // 无法计算耗时
		}
		instance.NodeStates[nodeID] = state
	}
	e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)
}

func (e *FlowExecutor) executeNode(ctx context.Context, instance *model.FlowInstance, nodes []model.FlowNode, edges []model.FlowEdge, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 执行节点 %s (%s)", shortID(instance), node.ID, node.Type)

	// 更新当前节点
	instance.CurrentNodeID = node.ID
	e.instanceRepo.Update(ctx, instance)

	// 获取节点名称
	nodeName := node.Name
	if nodeName == "" {
		if node.Config != nil {
			if label, ok := node.Config["label"].(string); ok {
				nodeName = label
			}
		}
	}

	// 发布 node_start 事件
	e.eventBus.PublishNodeStart(instance.ID, node.ID, node.Type, nodeName)

	// 标记节点为 running（自管理分支的节点由各自方法处理）
	if node.Type != model.NodeTypeApproval && node.Type != model.NodeTypeExecution && node.Type != model.NodeTypeCondition {
		e.setNodeState(ctx, instance, node.ID, "running", "")
	}

	// 为节点执行设置超时，防止永久挂起
	nodeCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// 根据节点类型执行
	var err error
	switch node.Type {
	case model.NodeTypeStart:
		// 起始节点，记录日志并继续
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
		e.logNode(nodeCtx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "流程开始", logDetails)
		err = nil

	case model.NodeTypeEnd:
		// 结束节点，记录日志并完成流程
		logDetails := map[string]interface{}{
			"input":   instance.Context,
			"process": []string{"流程执行完毕"},
			"output":  nil,
		}
		e.logNode(nodeCtx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "流程结束", logDetails)
		e.setNodeState(nodeCtx, instance, node.ID, "completed", "")
		e.eventBus.PublishNodeComplete(instance.ID, node.ID, node.Type, model.NodeStatusSuccess, nil, nil, nil, "")
		return e.complete(nodeCtx, instance)

	case model.NodeTypeHostExtractor:
		err = e.executeHostExtractor(nodeCtx, instance, node)

	case model.NodeTypeCMDBValidator:
		err = e.executeCMDBValidator(nodeCtx, instance, node)

	case model.NodeTypeApproval:
		// 审批节点需要等待，自己管理 node_states
		return e.executeApproval(nodeCtx, instance, node)

	case model.NodeTypeExecution:
		// 执行节点自己决定分支（success/partial/failed），自己管理 node_states
		return e.executeExecutionWithBranch(nodeCtx, instance, nodes, edges, node)

	case model.NodeTypeNotification:
		err = e.executeNotification(nodeCtx, instance, node)

	case model.NodeTypeCondition:
		// 条件节点需要特殊处理，自己管理 node_states
		return e.executeCondition(nodeCtx, instance, nodes, edges, node)

	case model.NodeTypeSetVariable:
		err = e.executeSetVariable(nodeCtx, instance, node)

	case model.NodeTypeCompute:
		err = e.executeCompute(nodeCtx, instance, node)

	default:
		logger.Exec("NODE").Warn("未知节点类型: %s", node.Type)
	}

	if err != nil {
		// 记录节点失败状态
		e.setNodeState(nodeCtx, instance, node.ID, "failed", err.Error())
		// 发布失败事件
		e.eventBus.PublishNodeComplete(instance.ID, node.ID, node.Type, model.NodeStatusFailed, nil, nil, nil, "")
		e.fail(nodeCtx, instance, "节点执行失败: "+err.Error())
		return err
	}

	// 记录节点成功状态
	e.setNodeState(nodeCtx, instance, node.ID, "completed", "")
	// 发布成功事件
	e.eventBus.PublishNodeComplete(instance.ID, node.ID, node.Type, model.NodeStatusSuccess, nil, nil, nil, "default")

	// 找到下一个节点
	nextNode := e.findNextNode(nodes, edges, node.ID)
	if nextNode == nil {
		return e.complete(ctx, instance)
	}

	// 继续执行下一个节点
	return e.executeNode(ctx, instance, nodes, edges, nextNode)
}

// parseFlowSnapshot 从实例快照解析节点和边
func (e *FlowExecutor) parseFlowSnapshot(instance *model.FlowInstance) ([]model.FlowNode, []model.FlowEdge, error) {
	var nodes []model.FlowNode
	var edges []model.FlowEdge

	nodesData, _ := json.Marshal(instance.FlowNodes)
	if err := json.Unmarshal(nodesData, &nodes); err != nil {
		return nil, nil, err
	}

	edgesData, _ := json.Marshal(instance.FlowEdges)
	if err := json.Unmarshal(edgesData, &edges); err != nil {
		return nil, nil, err
	}

	return nodes, edges, nil
}

// findStartNode 找到起始节点
func (e *FlowExecutor) findStartNode(nodes []model.FlowNode) *model.FlowNode {
	for i := range nodes {
		if nodes[i].Type == model.NodeTypeStart {
			return &nodes[i]
		}
	}
	return nil
}

// findNextNode 找到下一个节点
func (e *FlowExecutor) findNextNode(nodes []model.FlowNode, edges []model.FlowEdge, currentNodeID string) *model.FlowNode {
	// 找到从当前节点出发的边
	for _, edge := range edges {
		if edge.GetFrom() == currentNodeID {
			// 找到目标节点
			for i := range nodes {
				if nodes[i].ID == edge.GetTo() {
					return &nodes[i]
				}
			}
		}
	}
	return nil
}

// findNextNodeByHandle 根据输出口ID找到下一个节点
// handle: 输出口ID，如 "success", "failed", "approved", "rejected", "true", "false" 等
// 如果没有匹配的边，尝试查找 "default" 分支
func (e *FlowExecutor) findNextNodeByHandle(nodes []model.FlowNode, edges []model.FlowEdge, currentNodeID string, handle string) *model.FlowNode {
	// 先精确匹配指定的 handle
	for _, edge := range edges {
		if edge.GetFrom() == currentNodeID && edge.GetSourceHandle() == handle {
			for i := range nodes {
				if nodes[i].ID == edge.GetTo() {
					logger.Exec("FLOW").Debug("找到分支 %s -> %s (handle=%s)", currentNodeID, nodes[i].ID, handle)
					return &nodes[i]
				}
			}
		}
	}

	// 如果没有找到，尝试 default 分支（向后兼容）
	// 注意：rejected 和 failed 等负向分支不应 fallback 到 default，否则会导致拒绝后继续执行
	if handle != "default" && handle != "rejected" && handle != "failed" {
		for _, edge := range edges {
			if edge.GetFrom() == currentNodeID && edge.GetSourceHandle() == "default" {
				for i := range nodes {
					if nodes[i].ID == edge.GetTo() {
						logger.Exec("FLOW").Debug("回退到 default 分支 %s -> %s", currentNodeID, nodes[i].ID)
						return &nodes[i]
					}
				}
			}
		}
	}

	// 最后尝试无 handle 的边（仅在请求的 handle 也为空时才兼容旧数据）
	// 当有明确的 handle（如 "failed", "partial"）时，不应 fallback 到无标记的边
	if handle == "" || handle == "default" || handle == "success" {
		for _, edge := range edges {
			if edge.GetFrom() == currentNodeID && edge.SourceHandle == "" {
				for i := range nodes {
					if nodes[i].ID == edge.GetTo() {
						logger.Exec("FLOW").Debug("使用无 handle 的边 %s -> %s", currentNodeID, nodes[i].ID)
						return &nodes[i]
					}
				}
			}
		}
	}

	return nil
}

// complete 完成流程
func (e *FlowExecutor) complete(ctx context.Context, instance *model.FlowInstance) error {
	logger.Exec("FLOW").Info("[%s] 流程实例完成", instance.ID.String()[:8])

	// 发布 flow_complete 事件
	e.eventBus.PublishFlowComplete(instance.ID, true, model.FlowInstanceStatusCompleted, "流程执行完成")

	// 更新关联工单的自愈状态为 healed
	if instance.IncidentID != nil {
		if incident, err := e.incidentRepo.GetByID(ctx, *instance.IncidentID); err == nil {
			incident.HealingStatus = "healed"
			e.incidentRepo.Update(ctx, incident)
			logger.Exec("FLOW").Info("[%s] 工单 %s 自愈状态已更新为 healed", instance.ID.String()[:8], incident.ID.String()[:8])
		}
	}

	return e.instanceRepo.UpdateStatus(ctx, instance.ID, model.FlowInstanceStatusCompleted, "")
}

// fail 失败流程
func (e *FlowExecutor) fail(ctx context.Context, instance *model.FlowInstance, errMsg string) {
	logger.Exec("FLOW").Error("[%s] 流程实例失败: %s", instance.ID.String()[:8], errMsg)

	// 发布 flow_complete 事件
	e.eventBus.PublishFlowComplete(instance.ID, false, model.FlowInstanceStatusFailed, errMsg)

	// 更新关联工单的自愈状态为 failed
	if instance.IncidentID != nil {
		if incident, err := e.incidentRepo.GetByID(ctx, *instance.IncidentID); err == nil {
			incident.HealingStatus = "failed"
			e.incidentRepo.Update(ctx, incident)
			logger.Exec("FLOW").Info("[%s] 工单 %s 自愈状态已更新为 failed", instance.ID.String()[:8], incident.ID.String()[:8])
		}
	}

	e.instanceRepo.UpdateStatus(ctx, instance.ID, model.FlowInstanceStatusFailed, errMsg)
}

// executeHostExtractor 执行主机提取节点
// 支持 4 种提取模式: direct, split, regex, json_path
func (e *FlowExecutor) executeHostExtractor(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 执行主机提取节点", shortID(instance))

	// 从节点配置获取参数
	config := node.Config
	sourceField := "affected_ci" // 默认字段
	if sf, ok := config["source_field"].(string); ok && sf != "" {
		sourceField = sf
	}
	extractMode := "direct" // 默认模式
	if em, ok := config["extract_mode"].(string); ok && em != "" {
		extractMode = em
	}
	outputKey := "hosts" // 默认输出 key
	if v, _ := config["output_key"].(string); v != "" {
		outputKey = v
	}

	// 从 incident 获取源字段值（支持嵌套字段如 raw_data.cmdb_ci）
	var sourceValue string
	if instance.Context != nil {
		if incident, ok := instance.Context["incident"].(map[string]interface{}); ok {
			// 支持嵌套字段，使用 . 分隔
			parts := strings.Split(sourceField, ".")
			var current interface{} = incident

			for _, part := range parts {
				if currentMap, ok := current.(map[string]interface{}); ok {
					current = currentMap[part]
				} else {
					current = nil
					break
				}
			}

			if current != nil {
				switch v := current.(type) {
				case string:
					sourceValue = v
				default:
					// 尝试 JSON 序列化
					if jsonBytes, err := json.Marshal(v); err == nil {
						sourceValue = string(jsonBytes)
					}
				}
			}
		}
	}

	if sourceValue == "" {
		err := fmt.Errorf("源字段 %s 为空", sourceField)
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "提取主机失败", map[string]interface{}{
			"error":        err.Error(),
			"source_field": sourceField,
		})
		return err
	}

	// 根据提取模式提取主机
	var hosts []string
	var extractErr error

	switch extractMode {
	case "direct":
		// 直接使用整个字段值
		hosts = []string{strings.TrimSpace(sourceValue)}

	case "split":
		// 按分隔符拆分
		splitBy := ","
		if sb, ok := config["split_by"].(string); ok && sb != "" {
			splitBy = sb
		}
		parts := strings.Split(sourceValue, splitBy)
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				hosts = append(hosts, p)
			}
		}

	case "regex":
		// 正则表达式提取
		pattern := ""
		if rp, ok := config["regex_pattern"].(string); ok {
			pattern = rp
		}
		if pattern == "" {
			extractErr = fmt.Errorf("正则模式下必须指定 regex_pattern")
		} else {
			re, err := regexp.Compile(pattern)
			if err != nil {
				extractErr = fmt.Errorf("正则表达式编译失败: %v", err)
			} else {
				matches := re.FindAllStringSubmatch(sourceValue, -1)
				regexGroup := 0
				if rg, ok := config["regex_group"].(float64); ok {
					regexGroup = int(rg)
				}
				for _, match := range matches {
					if regexGroup < len(match) {
						host := strings.TrimSpace(match[regexGroup])
						if host != "" {
							hosts = append(hosts, host)
						}
					}
				}
			}
		}

	case "json_path":
		// JSON Path 提取（简化实现，支持基础路径）
		jsonPath := ""
		if jp, ok := config["json_path"].(string); ok {
			jsonPath = jp
		}
		if jsonPath == "" {
			extractErr = fmt.Errorf("json_path 模式下必须指定 json_path")
		} else {
			// 尝试解析 JSON
			var jsonData interface{}
			if err := json.Unmarshal([]byte(sourceValue), &jsonData); err != nil {
				extractErr = fmt.Errorf("JSON 解析失败: %v", err)
			} else {
				// 简化的 JSON Path 支持（目前只支持数组遍历）
				if arr, ok := jsonData.([]interface{}); ok {
					for _, item := range arr {
						if str, ok := item.(string); ok {
							hosts = append(hosts, str)
						} else if obj, ok := item.(map[string]interface{}); ok {
							// 尝试获取 name 或 host 字段
							if name, ok := obj["name"].(string); ok {
								hosts = append(hosts, name)
							} else if host, ok := obj["host"].(string); ok {
								hosts = append(hosts, host)
							}
						}
					}
				} else if str, ok := jsonData.(string); ok {
					hosts = []string{str}
				}
			}
		}

	default:
		extractErr = fmt.Errorf("不支持的提取模式: %s", extractMode)
	}

	if extractErr != nil {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "提取主机失败", map[string]interface{}{
			"error":        extractErr.Error(),
			"extract_mode": extractMode,
			"source_field": sourceField,
			"source_value": sourceValue,
		})
		return extractErr
	}

	if len(hosts) == 0 {
		err := fmt.Errorf("未提取到任何主机")
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelWarn, "未提取到主机", map[string]interface{}{
			"source_field": sourceField,
			"source_value": sourceValue,
			"extract_mode": extractMode,
		})
		return err
	}

	// 存入 context
	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}
	instance.Context[outputKey] = hosts

	// 更新实例
	if err := e.instanceRepo.Update(ctx, instance); err != nil {
		logger.Exec("FLOW").Error("更新实例失败: %v", err)
	}

	// 记录日志
	logDetails := map[string]interface{}{
		"input": map[string]interface{}{
			"context":      instance.Context,
			"source_field": sourceField,
		},
		"process": []string{
			fmt.Sprintf("读取配置 source_field: %s, extract_mode: %s", sourceField, extractMode),
			fmt.Sprintf("从工单数据提取源值: %s", sourceValue),
			fmt.Sprintf("使用 %s 模式提取主机", extractMode),
			fmt.Sprintf("成功提取 %d 个主机: %v", len(hosts), hosts),
			fmt.Sprintf("写入上下文 %s", outputKey),
		},
		"output": map[string]interface{}{
			outputKey: hosts,
		},
	}
	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "主机提取成功", logDetails)

	// 将提取结果写入 node_states，供前端展示
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	if existing, ok := instance.NodeStates[node.ID].(map[string]interface{}); ok {
		existing["extracted_hosts"] = hosts
		existing["source_field"] = sourceField
		existing["extract_mode"] = extractMode
		existing["host_count"] = len(hosts)
		instance.NodeStates[node.ID] = existing
	} else {
		instance.NodeStates[node.ID] = map[string]interface{}{
			"extracted_hosts": hosts,
			"source_field":    sourceField,
			"extract_mode":    extractMode,
			"host_count":      len(hosts),
		}
	}
	e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)

	logger.Exec("NODE").Info("[%s] 提取到 %d 个主机: %v", shortID(instance), len(hosts), hosts)
	return nil
}

// logNode 记录节点执行日志
func (e *FlowExecutor) logNode(ctx context.Context, instanceID uuid.UUID, nodeID, nodeType, level, message string, details map[string]interface{}) {
	logEntry := &model.FlowExecutionLog{
		FlowInstanceID: instanceID,
		NodeID:         nodeID,
		NodeType:       nodeType,
		Level:          level,
		Message:        message,
		Details:        details,
	}
	if err := e.flowLogRepo.Create(ctx, logEntry); err != nil {
		logger.Exec("FLOW").Error("记录日志失败: %v", err)
	}

	// 同时发布 SSE 事件
	e.eventBus.PublishNodeLog(instanceID, nodeID, nodeType, level, message, details)
}

// executeCondition 执行条件判断节点
// 根据配置中的条件表达式求值，选择不同的分支继续执行
func (e *FlowExecutor) executeCondition(ctx context.Context, instance *model.FlowInstance, nodes []model.FlowNode, edges []model.FlowEdge, node *model.FlowNode) error {
	logger.Exec("NODE").Debug("执行条件节点 %s", node.ID)

	// 标记节点为 running
	e.setNodeState(ctx, instance, node.ID, "running", "")

	config := node.Config

	// 解析条件配置
	// 格式: { "conditions": [{"expression": "...", "target": "node_id"}, ...], "default_target": "node_id" }
	var conditions []map[string]interface{}
	if conds, ok := config["conditions"].([]interface{}); ok {
		for _, c := range conds {
			if condMap, ok := c.(map[string]interface{}); ok {
				conditions = append(conditions, condMap)
			}
		}
	}

	defaultTarget := ""
	if dt, ok := config["default_target"].(string); ok {
		defaultTarget = dt
	}

	// 获取上下文用于求值
	context := instance.Context
	if context == nil {
		context = make(model.JSON)
	}

	// 遍历条件，找到第一个匹配的
	var matchedTarget string
	var matchedExpression string
	for _, cond := range conditions {
		expression, _ := cond["expression"].(string)
		target, _ := cond["target"].(string)

		if expression == "" || target == "" {
			continue
		}

		// 求值表达式
		result := e.evaluateCondition(expression, context, instance.NodeStates)
		logger.Exec("NODE").Debug("条件求值: %s = %v", expression, result)

		if result {
			matchedTarget = target
			matchedExpression = expression
			e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "条件匹配", map[string]interface{}{
				"expression":     expression,
				"matched_target": target,
			})
			break
		}
	}

	// 如果没有匹配的条件，使用默认目标
	if matchedTarget == "" {
		matchedTarget = defaultTarget
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "使用默认分支", map[string]interface{}{
			"default_target": defaultTarget,
		})
	}

	if matchedTarget == "" {
		errMsg := "条件节点没有匹配的分支且无默认目标"
		e.setNodeState(ctx, instance, node.ID, "failed", errMsg)
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "条件判断失败", map[string]interface{}{
			"error": errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	// 找到目标节点
	var nextNode *model.FlowNode
	for i := range nodes {
		if nodes[i].ID == matchedTarget {
			nextNode = &nodes[i]
			break
		}
	}

	if nextNode == nil {
		errMsg := fmt.Sprintf("找不到目标节点: %s", matchedTarget)
		e.setNodeState(ctx, instance, node.ID, "failed", errMsg)
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "条件判断失败", map[string]interface{}{
			"error":  errMsg,
			"target": matchedTarget,
		})
		return fmt.Errorf("%s", errMsg)
	}

	// 标记条件节点为完成
	e.setNodeState(ctx, instance, node.ID, "completed", "")

	// 将分支决策结果写入 node_states
	if existing, ok := instance.NodeStates[node.ID].(map[string]interface{}); ok {
		existing["activated_branch"] = matchedTarget
		if matchedExpression != "" {
			existing["matched_expression"] = matchedExpression
		}
		instance.NodeStates[node.ID] = existing
		e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)
	}

	logger.Exec("NODE").Info("条件节点跳转到: %s", matchedTarget)

	// 继续执行目标节点
	return e.executeNode(ctx, instance, nodes, edges, nextNode)
}

// executeSetVariable 执行变量设置节点
// 将配置中的变量值设置到 context 中，供后续节点使用
func (e *FlowExecutor) executeSetVariable(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Debug("执行变量设置节点 %s", node.ID)

	config := node.Config

	// 解析变量配置
	// 格式: { "variables": {"var_name": "value_or_expression", ...} }
	variables, ok := config["variables"].(map[string]interface{})
	if !ok {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelWarn, "变量配置为空", nil)
		return nil
	}

	// 确保 context 存在
	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}

	setVars := make(map[string]interface{})

	for varName, varValue := range variables {
		var finalValue interface{}

		switch v := varValue.(type) {
		case string:
			// 检查是否是表达式（包含 . 或比较操作符）
			if strings.Contains(v, ".") || strings.Contains(v, "==") || strings.Contains(v, "!=") ||
				strings.Contains(v, ">") || strings.Contains(v, "<") {
				// 尝试作为表达式求值
				if strings.Contains(v, "==") || strings.Contains(v, "!=") ||
					strings.Contains(v, ">") || strings.Contains(v, "<") {
					// 布尔表达式
					finalValue = e.evaluateCondition(v, instance.Context, instance.NodeStates)
				} else {
					// 变量引用，解析路径
					resolved := e.resolveValue(v, instance.Context, instance.NodeStates)
					if resolved != nil {
						finalValue = resolved
					} else {
						finalValue = v // 保持原值
					}
				}
			} else {
				// 普通字符串值
				finalValue = v
			}
		default:
			// 其他类型（数字、布尔、对象）直接使用
			finalValue = v
		}

		instance.Context[varName] = finalValue
		setVars[varName] = finalValue
		logger.Exec("NODE").Debug("设置变量: %s = %v", varName, finalValue)
	}

	// 更新实例
	if err := e.instanceRepo.Update(ctx, instance); err != nil {
		return err
	}

	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "变量设置完成", map[string]interface{}{
		"variables": setVars,
	})

	// 将设置的变量写入 node_states
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	if existing, ok := instance.NodeStates[node.ID].(map[string]interface{}); ok {
		existing["variables_set"] = setVars
		instance.NodeStates[node.ID] = existing
		e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)
	}

	return nil
}

// evaluateCondition 求值条件表达式（简化实现）
// 支持格式:
// - "execution_result.exit_code == 0"
// - "execution_result.success == true"
// - "validated_hosts.length > 0"
func (e *FlowExecutor) evaluateCondition(expression string, context model.JSON, nodeStates model.JSON) bool {
	expression = strings.TrimSpace(expression)

	// 解析表达式: left operator right
	var left, operator, right string
	operators := []string{"==", "!=", ">=", "<=", ">", "<"}
	for _, op := range operators {
		if parts := strings.SplitN(expression, op, 2); len(parts) == 2 {
			left = strings.TrimSpace(parts[0])
			operator = op
			right = strings.TrimSpace(parts[1])
			break
		}
	}

	if operator == "" {
		logger.Exec("NODE").Warn("无法解析条件表达式: %s", expression)
		return false
	}

	// 获取左值
	leftValue := e.resolveValue(left, context, nodeStates)

	// 解析右值
	rightValue := e.parseRightValue(right)

	logger.Exec("NODE").Debug("比较: %v %s %v", leftValue, operator, rightValue)

	// 执行比较
	return e.compare(leftValue, operator, rightValue)
}

// resolveValue 从 context 或 nodeStates 解析变量值
func (e *FlowExecutor) resolveValue(path string, context model.JSON, nodeStates model.JSON) interface{} {
	parts := strings.Split(path, ".")

	// 尝试从 nodeStates 解析 (如 execution_result.exit_code)
	if len(parts) > 0 {
		// 检查 nodeStates 中的执行结果
		if parts[0] == "execution_result" || parts[0] == "execution" {
			// 从 context 的 execution_result 获取（优先，更可靠）
			if execResult, ok := context["execution_result"].(map[string]interface{}); ok {
				if len(parts) > 1 {
					return e.getNestedValue(execResult, parts[1:])
				}
				return execResult
			}
			// 回退到 nodeStates 中查找类型为 execution 的节点（精确匹配节点类型，而非子串）
			for nodeID, stateRaw := range nodeStates {
				if state, ok := stateRaw.(map[string]interface{}); ok {
					// 只匹配以 exec_ 开头的节点 ID 或精确匹配 execution
					if strings.HasPrefix(nodeID, "exec_") || nodeID == "execution" {
						if len(parts) > 1 {
							return e.getNestedValue(state, parts[1:])
						}
						return state
					}
				}
			}
		}

		// 尝试从 context 解析
		if val, ok := context[parts[0]]; ok {
			if len(parts) == 1 {
				return val
			}
			if mapVal, ok := val.(map[string]interface{}); ok {
				return e.getNestedValue(mapVal, parts[1:])
			}
		}
	}

	return nil
}

// getNestedValue 获取嵌套的值
func (e *FlowExecutor) getNestedValue(data map[string]interface{}, path []string) interface{} {
	if len(path) == 0 {
		return data
	}

	key := path[0]

	// 支持 .length 获取数组/map长度
	if key == "length" {
		return len(data)
	}

	val, ok := data[key]
	if !ok {
		return nil
	}

	if len(path) == 1 {
		return val
	}

	if mapVal, ok := val.(map[string]interface{}); ok {
		return e.getNestedValue(mapVal, path[1:])
	}

	return nil
}

// parseRightValue 解析右值（支持字面量）
func (e *FlowExecutor) parseRightValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// 布尔值
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// 字符串（带引号）
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1]
	}

	// 数字
	if strings.Contains(s, ".") {
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
			return f
		}
	} else {
		var i int64
		if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
			return i
		}
	}

	return s
}

// compare 执行比较操作
func (e *FlowExecutor) compare(left interface{}, op string, right interface{}) bool {
	switch op {
	case "==":
		// 先尝试类型一致的比较
		if lb, ok := left.(bool); ok {
			if rb, ok := right.(bool); ok {
				return lb == rb
			}
		}
		// 统一转为字符串比较（解决类型不对称问题）
		ls := fmt.Sprintf("%v", left)
		rs := fmt.Sprintf("%v", right)
		if ls == rs {
			return true
		}
		// 回退到数值比较（仅当两边都能转为有意义的数字时）
		leftNum := toFloat(left)
		rightNum := toFloat(right)
		if leftNum != 0 || rightNum != 0 {
			return leftNum == rightNum
		}
		return false
	case "!=":
		if lb, ok := left.(bool); ok {
			if rb, ok := right.(bool); ok {
				return lb != rb
			}
		}
		ls := fmt.Sprintf("%v", left)
		rs := fmt.Sprintf("%v", right)
		if ls != rs {
			return true
		}
		leftNum := toFloat(left)
		rightNum := toFloat(right)
		if leftNum != 0 || rightNum != 0 {
			return leftNum != rightNum
		}
		return false
	case ">":
		return toFloat(left) > toFloat(right)
	case "<":
		return toFloat(left) < toFloat(right)
	case ">=":
		return toFloat(left) >= toFloat(right)
	case "<=":
		return toFloat(left) <= toFloat(right)
	}

	return false
}

// executeCMDBValidator 执行 CMDB 校验节点
// 查询内部 CMDB 表验证主机，获取真实 IP
func (e *FlowExecutor) executeCMDBValidator(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 执行 CMDB 校验节点", shortID(instance))

	// 从节点配置获取参数
	config := node.Config
	inputKey := "hosts" // 默认从 hosts 读取
	if ik, ok := config["input_key"].(string); ok && ik != "" {
		inputKey = ik
	}
	outputKey := "validated_hosts" // 默认输出 key
	if ok, _ := config["output_key"].(string); ok != "" {
		outputKey = ok
	}
	failOnNotFound := true // 默认找不到主机则失败
	if fn, ok := config["fail_on_not_found"].(bool); ok {
		failOnNotFound = fn
	}
	skipMissing := false // 默认不跳过缺失的主机
	if sm, ok := config["skip_missing"].(bool); ok {
		skipMissing = sm
	}

	// 从 context 获取主机列表
	var hosts []string
	if instance.Context != nil {
		if hostList, ok := instance.Context[inputKey].([]interface{}); ok {
			for _, h := range hostList {
				if hostStr, ok := h.(string); ok {
					hosts = append(hosts, hostStr)
				}
			}
		} else if hostList, ok := instance.Context[inputKey].([]string); ok {
			hosts = hostList
		}
	}

	if len(hosts) == 0 {
		err := fmt.Errorf("未找到主机列表，input_key=%s", inputKey)
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "CMDB 验证失败", map[string]interface{}{
			"error":     err.Error(),
			"input_key": inputKey,
		})
		return err
	}

	// 验证每个主机
	var validatedHosts []map[string]interface{}
	var invalidHosts []map[string]interface{}

	for _, host := range hosts {
		logger.Exec("NODE").Debug("验证主机: %s", host)

		// 查询内部 CMDB
		cmdbItem, err := e.cmdbRepo.FindByNameOrIP(ctx, host)
		if err != nil {
			// 主机不存在
			logger.Exec("NODE").Warn("主机未在 CMDB 找到: %s", host)
			invalidHosts = append(invalidHosts, map[string]interface{}{
				"original_name": host,
				"valid":         false,
				"reason":        "not_found_in_cmdb",
			})

			if !skipMissing && failOnNotFound {
				e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "CMDB 验证失败", map[string]interface{}{
					"host":   host,
					"reason": "not_found_in_cmdb",
				})
				return fmt.Errorf("主机 %s 未在 CMDB 找到", host)
			}
			continue
		}

		// 检查状态（maintenance 和 offline 都不允许执行）
		if cmdbItem.Status == "maintenance" || cmdbItem.Status == "offline" {
			reason := "maintenance_status"
			if cmdbItem.Status == "offline" {
				reason = "offline_status"
			}
			logger.Exec("NODE").Warn("主机状态异常: %s, status=%s", host, cmdbItem.Status)
			invalidHosts = append(invalidHosts, map[string]interface{}{
				"original_name":      host,
				"valid":              false,
				"reason":             reason,
				"status":             cmdbItem.Status,
				"maintenance_reason": cmdbItem.MaintenanceReason,
			})

			if !skipMissing && failOnNotFound {
				e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "CMDB 验证失败", map[string]interface{}{
					"host":   host,
					"reason": reason,
					"status": cmdbItem.Status,
				})
				return fmt.Errorf("主机 %s 状态异常: %s", host, cmdbItem.Status)
			}
			continue
		}

		// 验证成功
		ipAddress := cmdbItem.IPAddress
		if ipAddress == "" {
			// 如果没有 IP，使用原始名称
			ipAddress = host
		}

		validatedHost := map[string]interface{}{
			"original_name": host,
			"ip_address":    ipAddress,
			"name":          cmdbItem.Name,
			"hostname":      cmdbItem.Hostname,
			"status":        cmdbItem.Status,
			"environment":   cmdbItem.Environment,
			"os":            cmdbItem.OS,
			"os_version":    cmdbItem.OSVersion,
			"owner":         cmdbItem.Owner,
			"location":      cmdbItem.Location,
			"valid":         true,
			"cmdb_id":       cmdbItem.ID.String(),
		}
		validatedHosts = append(validatedHosts, validatedHost)
		logger.Exec("NODE").Info("[%s] 主机验证成功: %s -> IP=%s", shortID(instance), host, ipAddress)
	}

	// 检查是否有有效主机
	if len(validatedHosts) == 0 {
		err := fmt.Errorf("没有任何主机通过 CMDB 验证")
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "CMDB 验证失败", map[string]interface{}{
			"input_hosts":   hosts,
			"invalid_hosts": invalidHosts,
		})
		return err
	}

	// 存入 context
	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}
	instance.Context[outputKey] = validatedHosts
	instance.Context["validation_summary"] = map[string]interface{}{
		"total":   len(hosts),
		"valid":   len(validatedHosts),
		"invalid": len(invalidHosts),
	}
	if len(invalidHosts) > 0 {
		instance.Context["invalid_hosts"] = invalidHosts
	}

	// 更新实例
	if err := e.instanceRepo.Update(ctx, instance); err != nil {
		logger.Exec("FLOW").Error("更新实例失败: %v", err)
	}

	// 记录日志
	processLogs := []string{
		fmt.Sprintf("读取配置 input_key: %s, output_key: %s", inputKey, outputKey),
		fmt.Sprintf("从上下文获取 %d 个主机", len(hosts)),
		"开始查询 CMDB 数据库",
	}
	for _, vh := range validatedHosts {
		processLogs = append(processLogs, fmt.Sprintf("主机 %v: 验证通过", vh["original_name"]))
	}
	for _, ih := range invalidHosts {
		processLogs = append(processLogs, fmt.Sprintf("主机 %v: %v", ih["original_name"], ih["reason"]))
	}
	processLogs = append(processLogs, fmt.Sprintf("验证完成: %d 通过, %d 失败", len(validatedHosts), len(invalidHosts)))
	processLogs = append(processLogs, fmt.Sprintf("写入上下文 %s", outputKey))

	logDetails := map[string]interface{}{
		"input": map[string]interface{}{
			"hosts":     hosts,
			"input_key": inputKey,
		},
		"process": processLogs,
		"output": map[string]interface{}{
			outputKey:       validatedHosts,
			"invalid_hosts": invalidHosts,
		},
	}
	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "CMDB 验证成功", logDetails)

	// 将验证结果写入 node_states
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	if existing, ok := instance.NodeStates[node.ID].(map[string]interface{}); ok {
		existing["validated_hosts"] = validatedHosts
		existing["invalid_hosts"] = invalidHosts
		existing["validation_summary"] = map[string]interface{}{
			"total":   len(hosts),
			"valid":   len(validatedHosts),
			"invalid": len(invalidHosts),
		}
		instance.NodeStates[node.ID] = existing
		e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)
	}

	logger.Exec("NODE").Info("[%s] CMDB 验证完成: %d/%d 个主机通过", shortID(instance), len(validatedHosts), len(hosts))
	return nil
}

// executeApproval 执行审批节点（等待人工审批）
func (e *FlowExecutor) executeApproval(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 创建审批任务", shortID(instance))

	config := node.Config

	// 从节点配置中获取审批配置
	timeoutHours := 24.0 // 默认 24 小时
	if config != nil {
		if t, ok := config["timeout_hours"].(float64); ok {
			timeoutHours = t
		}
	}
	timeout := time.Duration(timeoutHours) * time.Hour
	timeoutAt := time.Now().Add(timeout)

	// 获取审批者配置
	var approvers model.JSONArray
	var approverRoles model.JSONArray

	if config != nil {
		if a, ok := config["approvers"].([]interface{}); ok {
			approvers = a
		} else if a, ok := config["approvers"]; ok {
			approvers = model.JSONArray{a}
		}
		if r, ok := config["approver_roles"].([]interface{}); ok {
			approverRoles = r
		} else if r, ok := config["approver_roles"]; ok {
			approverRoles = model.JSONArray{r}
		}
	}

	// 获取审批标题和描述
	title := fmt.Sprintf("流程实例 %s 审批请求", instance.ID.String()[:8])
	description := ""
	if config != nil {
		if t, ok := config["title"].(string); ok && t != "" {
			title = t
		}
		if d, ok := config["description"].(string); ok {
			description = d
		}
	}

	// 创建审批任务
	task := &model.ApprovalTask{
		FlowInstanceID: instance.ID,
		NodeID:         node.ID,
		Status:         model.ApprovalTaskStatusPending,
		TimeoutAt:      &timeoutAt,
		Approvers:      approvers,
		ApproverRoles:  approverRoles,
	}

	if err := e.approvalRepo.Create(ctx, task); err != nil {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "创建审批任务失败", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// 更新实例状态为等待审批
	instance.Status = model.FlowInstanceStatusWaitingApproval
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	instance.NodeStates[node.ID] = map[string]interface{}{
		"status":      "waiting_approval",
		"task_id":     task.ID,
		"title":       title,
		"description": description,
		"timeout_at":  timeoutAt.Format(time.RFC3339),
		"created_at":  time.Now().Format(time.RFC3339),
		"started_at":  time.Now().Format(time.RFC3339),
	}
	e.instanceRepo.Update(ctx, instance)
	e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)

	// 记录日志
	logDetails := map[string]interface{}{
		"input": map[string]interface{}{
			"context": instance.Context,
		},
		"process": []string{
			fmt.Sprintf("读取配置 title: %s", title),
			fmt.Sprintf("审批人: %v, 审批角色: %v", approvers, approverRoles),
			fmt.Sprintf("超时时间: %.0f 小时", timeoutHours),
			fmt.Sprintf("创建审批任务 ID: %s", task.ID.String()[:8]),
			"流程暂停，等待审批",
		},
		"output": map[string]interface{}{
			"task_id":    task.ID,
			"timeout_at": timeoutAt.Format(time.RFC3339),
			"status":     "waiting_approval",
		},
	}
	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "等待审批", logDetails)

	logger.Exec("NODE").Info("[%s] 审批任务已创建，ID=%s", shortID(instance), task.ID.String()[:8])

	// 返回 nil 表示等待审批，流程会在 ResumeAfterApproval 中继续
	return nil
}

// executeExecution 执行任务节点
// 使用任务模板作为最小执行单位，直接调用 execution.Service
func (e *FlowExecutor) executeExecution(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("ANSIBLE").Info("[%s] 执行任务节点", shortID(instance))
	startTime := time.Now()

	// 初始化节点状态
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}

	// ==================== 步骤 1: 解析节点配置 ====================
	config := node.Config

	// 任务模板 ID（必须指定）
	taskTemplateID := ""
	if tid, ok := config["task_template_id"].(string); ok {
		taskTemplateID = tid
	}

	if taskTemplateID == "" {
		err := fmt.Errorf("必须指定 task_template_id")
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "配置错误", map[string]interface{}{
			"error": err.Error(),
		})
		instance.NodeStates[node.ID] = map[string]interface{}{
			"status":  "failed",
			"message": err.Error(),
		}
		e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)
		return err
	}

	// 解析任务模板 ID
	taskUUID, err := uuid.Parse(taskTemplateID)
	if err != nil {
		errMsg := fmt.Errorf("无效的 task_template_id: %v", err)
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "配置错误", map[string]interface{}{
			"task_template_id": taskTemplateID,
			"error":            err.Error(),
		})
		instance.NodeStates[node.ID] = map[string]interface{}{
			"status":  "failed",
			"message": errMsg.Error(),
		}
		e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)
		return errMsg
	}

	// ==================== 步骤 2: 获取目标主机 ====================
	// 优先从 context 获取已验证的主机
	var targetHosts string

	hostsKey := "validated_hosts"
	if hk, ok := config["hosts_key"].(string); ok && hk != "" {
		hostsKey = hk
	}

	var hostIPs []string
	if instance.Context != nil {
		hostsData := instance.Context[hostsKey]
		switch list := hostsData.(type) {
		case []string:
			// host_extractor 的 split/direct 模式直接存储 []string
			for _, h := range list {
				if h != "" {
					hostIPs = append(hostIPs, h)
				}
			}
		case []interface{}:
			// JSON 反序列化后的数组 或 从 DB 加载的数据
			for _, h := range list {
				if hostMap, ok := h.(map[string]interface{}); ok {
					if ip, ok := hostMap["ip_address"].(string); ok && ip != "" {
						hostIPs = append(hostIPs, ip)
					} else if name, ok := hostMap["original_name"].(string); ok {
						hostIPs = append(hostIPs, name)
					}
				} else if hostStr, ok := h.(string); ok && hostStr != "" {
					hostIPs = append(hostIPs, hostStr)
				}
			}
		case []map[string]interface{}:
			// cmdb_validator 的输出格式
			for _, hostMap := range list {
				if ip, ok := hostMap["ip_address"].(string); ok && ip != "" {
					hostIPs = append(hostIPs, ip)
				} else if name, ok := hostMap["original_name"].(string); ok {
					hostIPs = append(hostIPs, name)
				}
			}
		}

		// 兼容：尝试从 hosts 获取
		if len(hostIPs) == 0 {
			if hostList, ok := instance.Context["hosts"].([]string); ok {
				hostIPs = hostList
			} else if hostList, ok := instance.Context["hosts"].([]interface{}); ok {
				for _, h := range hostList {
					if hostStr, ok := h.(string); ok {
						hostIPs = append(hostIPs, hostStr)
					}
				}
			}
		}
	}

	if len(hostIPs) > 0 {
		targetHosts = strings.Join(hostIPs, ",")
		logger.Exec("ANSIBLE").Info("[%s] 使用 context 中的主机: %s", shortID(instance), targetHosts)
	}

	// ==================== 步骤 3: 准备额外变量 ====================
	// 3.1 从节点配置获取静态 extra_vars
	var mergedExtraVars map[string]any
	if nodeExtraVars, ok := config["extra_vars"].(map[string]interface{}); ok {
		mergedExtraVars = make(map[string]any)
		for k, v := range nodeExtraVars {
			mergedExtraVars[k] = v
		}
	} else {
		mergedExtraVars = make(map[string]any)
	}

	// 3.2 处理 variable_mappings - 从 context 动态提取变量
	// 配置格式:
	// {
	//   "variable_mappings": {
	//     "target_ip": "validated_hosts[0].ip_address",
	//     "service_name": "incident.affected_service",
	//     "host_list": "join(validated_hosts, ',')"
	//   }
	// }
	if variableMappings, ok := config["variable_mappings"].(map[string]interface{}); ok && len(variableMappings) > 0 {
		evaluator := NewExpressionEvaluator()
		logger.Exec("ANSIBLE").Info("[%s] 处理 variable_mappings: %d 个映射", shortID(instance), len(variableMappings))

		for varName, exprRaw := range variableMappings {
			expression, ok := exprRaw.(string)
			if !ok || expression == "" {
				logger.Exec("ANSIBLE").Warn("[%s] variable_mappings 中的 %s 值无效", shortID(instance), varName)
				continue
			}

			// 使用表达式引擎求值
			result, err := evaluator.Evaluate(expression, instance.Context)
			if err != nil {
				logger.Exec("ANSIBLE").Warn("[%s] 计算 %s 失败: %v (表达式: %s)", shortID(instance), varName, err, expression)
				continue
			}

			mergedExtraVars[varName] = result
			logger.Exec("ANSIBLE").Debug("[%s] 变量映射: %s = %v (来自: %s)", shortID(instance), varName, result, expression)
		}
	}

	// ==================== 步骤 4: 更新节点状态为运行中 ====================
	instance.NodeStates[node.ID] = map[string]interface{}{
		"status":       "running",
		"task_id":      taskTemplateID,
		"target_hosts": targetHosts,
		"started_at":   startTime.Format(time.RFC3339),
		"message":      "正在执行任务模板...",
	}
	e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)

	// 获取任务模板详细信息用于日志
	taskTemplateInfo := map[string]interface{}{
		"task_template_id": taskTemplateID,
	}
	task, err := e.executionRepo.GetTaskByID(ctx, taskUUID)
	if err == nil && task != nil {
		taskTemplateInfo["task_template_name"] = task.Name
		taskTemplateInfo["task_description"] = task.Description
		if task.PlaybookID != uuid.Nil {
			taskTemplateInfo["playbook_id"] = task.PlaybookID.String()
		}
		if task.ExtraVars != nil {
			taskTemplateInfo["template_vars"] = task.ExtraVars
		}
	}

	// 构建详细的执行日志
	processLogs := []string{
		"--- 任务模板配置 ---",
		fmt.Sprintf("任务模板 ID: %s", taskTemplateID),
	}
	if task != nil {
		processLogs = append(processLogs, fmt.Sprintf("任务模板名称: %s", task.Name))
		if task.Description != "" {
			processLogs = append(processLogs, fmt.Sprintf("任务描述: %s", task.Description))
		}
	}

	processLogs = append(processLogs, "--- 目标主机 ---")
	processLogs = append(processLogs, fmt.Sprintf("主机来源: 上下文变量 %s", hostsKey))
	processLogs = append(processLogs, fmt.Sprintf("主机列表: %s", targetHosts))

	processLogs = append(processLogs, "--- 变量配置 ---")
	if len(mergedExtraVars) > 0 {
		processLogs = append(processLogs, fmt.Sprintf("共 %d 个变量:", len(mergedExtraVars)))
		for k, v := range mergedExtraVars {
			processLogs = append(processLogs, fmt.Sprintf("  %s = %v", k, v))
		}
	} else {
		processLogs = append(processLogs, "无额外变量")
	}

	processLogs = append(processLogs, "--- 开始执行 ---")
	processLogs = append(processLogs, "调用执行服务...")

	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "开始执行", map[string]interface{}{
		"input": map[string]interface{}{
			"context":            instance.Context,
			"task_template_info": taskTemplateInfo,
			"target_hosts":       targetHosts,
			"merged_vars":        mergedExtraVars,
			"hosts_source":       hostsKey,
		},
		"process": processLogs,
	})

	// ==================== 步骤 5: 调用执行服务 ====================
	execOpts := &execution.ExecuteOptions{
		TriggeredBy:      "healing",
		TargetHosts:      targetHosts,
		ExtraVars:        mergedExtraVars,
		SkipNotification: true, // 自愈流程使用自己的通知节点
	}

	run, execErr := e.executionSvc.ExecuteTask(ctx, taskUUID, execOpts)

	// ==================== 步骤 6: 等待执行完成 ====================
	executionStatus := "completed"
	executionMessage := "执行成功"
	var runResult map[string]interface{}

	if execErr != nil {
		executionStatus = "failed"
		executionMessage = fmt.Sprintf("执行失败: %v", execErr)
	} else if run != nil {
		// 同步等待执行完成
		timeout := 30 * time.Minute
		completedRun, waitErr := e.waitForRunCompletion(ctx, run.ID, timeout)
		if waitErr != nil {
			executionStatus = "failed"
			executionMessage = fmt.Sprintf("等待执行完成失败: %v", waitErr)
		} else if completedRun != nil {
			runResult = map[string]interface{}{
				"run_id":    completedRun.ID.String(),
				"status":    completedRun.Status,
				"exit_code": completedRun.ExitCode,
				"stats":     completedRun.Stats,
			}
			switch completedRun.Status {
			case "failed":
				executionStatus = "failed"
				if completedRun.ExitCode != nil {
					executionMessage = fmt.Sprintf("任务执行失败 (退出码: %d)", *completedRun.ExitCode)
				} else {
					executionMessage = "任务执行失败"
				}
			case "partial":
				executionStatus = "partial"
				executionMessage = "任务部分成功（部分主机执行失败或不可达）"
			}
		}
	}

	duration := time.Since(startTime)

	// ==================== 步骤 7: 记录结果 ====================
	executionResult := map[string]interface{}{
		"status":       executionStatus,
		"message":      executionMessage,
		"started_at":   startTime.Format(time.RFC3339),
		"finished_at":  time.Now().Format(time.RFC3339),
		"duration_ms":  duration.Milliseconds(),
		"task_id":      taskTemplateID,
		"target_hosts": targetHosts,
	}
	if runResult != nil {
		executionResult["run"] = runResult
	}

	instance.NodeStates[node.ID] = executionResult
	e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)

	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}
	instance.Context["execution_result"] = executionResult
	e.instanceRepo.Update(ctx, instance)

	logLevel := model.LogLevelInfo
	switch executionStatus {
	case "failed":
		logLevel = model.LogLevelError
	case "partial":
		logLevel = model.LogLevelWarn
	}
	e.logNode(ctx, instance.ID, node.ID, node.Type, logLevel, executionMessage, executionResult)

	logger.Exec("ANSIBLE").Info("[%s] 执行完成，状态: %s", shortID(instance), executionStatus)

	if executionStatus == "failed" {
		return fmt.Errorf("%s", executionMessage)
	}
	if executionStatus == "partial" {
		return fmt.Errorf("%s", executionMessage)
	}

	return nil
}

// executeExecutionWithBranch 执行任务节点并根据结果选择分支
// 分支: success（全部成功）、partial（部分成功）、failed（全部失败或其他错误）
func (e *FlowExecutor) executeExecutionWithBranch(ctx context.Context, instance *model.FlowInstance, nodes []model.FlowNode, edges []model.FlowEdge, node *model.FlowNode) error {
	// 调用原有的执行逻辑（但不返回错误，改为记录状态）
	logger.Exec("ANSIBLE").Info("[%s] 执行任务节点（分支模式）", shortID(instance))

	// 标记节点为 running
	e.setNodeState(ctx, instance, node.ID, "running", "")

	// 执行任务
	err := e.executeExecution(ctx, instance, node)

	// 从 context 获取执行结果
	var outputHandle string
	if instance.Context != nil {
		if execResult, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			if status, ok := execResult["status"].(string); ok {
				// 根据执行状态映射到分支
				switch status {
				case "completed", "success":
					outputHandle = "success"
				case "partial":
					outputHandle = "partial"
				default:
					// 所有其他状态（failed, cancelled, timeout, error 等）都走 failed 分支
					outputHandle = "failed"
				}
			}
		}

		// 从 run 结果中获取更精确的状态
		if execResult, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			if runInfo, ok := execResult["run"].(map[string]interface{}); ok {
				if runStatus, ok := runInfo["status"].(string); ok {
					switch runStatus {
					case "success":
						outputHandle = "success"
					case "partial":
						outputHandle = "partial"
					default:
						outputHandle = "failed"
					}
				}
			}
		}
	}

	// 如果没有获取到状态，根据 error 判断
	if outputHandle == "" {
		if err != nil {
			outputHandle = "failed"
		} else {
			outputHandle = "success"
		}
	}

	logger.Exec("ANSIBLE").Info("[%s] 执行节点结果分支: %s", shortID(instance), outputHandle)

	// 记录节点状态
	nodeStateStatus := "completed"
	var nodeErrMsg string
	switch outputHandle {
	case "failed":
		nodeStateStatus = "failed"
		if err != nil {
			nodeErrMsg = err.Error()
		}
	case "partial":
		nodeStateStatus = "partial"
	}
	e.setNodeState(ctx, instance, node.ID, nodeStateStatus, nodeErrMsg)

	// 回写执行结果到 node_states（setNodeState 只写了状态元数据，把执行细节加回来）
	if existing, ok := instance.NodeStates[node.ID].(map[string]interface{}); ok {
		if execResult, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			if run, ok := execResult["run"]; ok {
				existing["run"] = run
			}
			if taskID, ok := execResult["task_id"]; ok {
				existing["task_id"] = taskID
			}
			if targetHosts, ok := execResult["target_hosts"]; ok {
				existing["target_hosts"] = targetHosts
			}
		}
		existing["output_handle"] = outputHandle
		instance.NodeStates[node.ID] = existing
		e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)
	}

	// 发布执行节点完成事件，包含分支信息
	nodeStatus := model.NodeStatusSuccess
	switch outputHandle {
	case "failed":
		nodeStatus = model.NodeStatusFailed
	case "partial":
		nodeStatus = model.NodeStatusPartial
	}
	e.eventBus.PublishNodeComplete(instance.ID, node.ID, node.Type, nodeStatus, nil, nil, nil, outputHandle)

	// 找到对应分支的下一个节点
	nextNode := e.findNextNodeByHandle(nodes, edges, node.ID, outputHandle)
	if nextNode == nil {
		// 如果没有找到对应分支，非 success 状态应让流程失败
		if outputHandle == "failed" {
			errMsg := "执行失败且无 failed 分支"
			if err != nil {
				errMsg += ": " + err.Error()
			}
			e.fail(ctx, instance, errMsg)
			return fmt.Errorf("%s", errMsg)
		}
		if outputHandle == "partial" {
			errMsg := "部分成功但无 partial 分支"
			if err != nil {
				errMsg += ": " + err.Error()
			}
			e.fail(ctx, instance, errMsg)
			return fmt.Errorf("%s", errMsg)
		}
		// success 且没有下一个节点，流程结束
		return e.complete(ctx, instance)
	}

	// 继续执行下一个节点
	return e.executeNode(ctx, instance, nodes, edges, nextNode)
}

// waitForRunCompletion 等待执行完成
func (e *FlowExecutor) waitForRunCompletion(ctx context.Context, runID uuid.UUID, timeout time.Duration) (*model.ExecutionRun, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("等待执行完成超时")
		}

		run, err := e.executionRepo.GetRunByID(ctx, runID)
		if err != nil {
			return nil, err
		}

		if run.Status == "success" || run.Status == "failed" || run.Status == "cancelled" || run.Status == "partial" {
			return run, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// executeNotification 执行通知节点
// 复用 notification 服务发送通知
func (e *FlowExecutor) executeNotification(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 执行通知节点", shortID(instance))

	// 从节点配置获取参数
	config := node.Config

	// 新格式：notification_configs - 每个渠道可以有不同的模板
	type channelTemplateConfig struct {
		ChannelID  string
		TemplateID string
	}
	var notificationConfigs []channelTemplateConfig

	// 优先使用新格式
	if configs, ok := config["notification_configs"].([]interface{}); ok && len(configs) > 0 {
		for _, cfg := range configs {
			if cfgMap, ok := cfg.(map[string]interface{}); ok {
				channelID, _ := cfgMap["channel_id"].(string)
				templateID, _ := cfgMap["template_id"].(string)
				if channelID != "" {
					notificationConfigs = append(notificationConfigs, channelTemplateConfig{
						ChannelID:  channelID,
						TemplateID: templateID,
					})
				}
			}
		}
	}

	// 向后兼容：如果没有新格式，使用旧格式（所有渠道共用一个模板）
	if len(notificationConfigs) == 0 {
		// 渠道 ID
		var channelIDs []string
		if cid, ok := config["channel_id"].(string); ok && cid != "" {
			channelIDs = append(channelIDs, cid)
		}
		if cids, ok := config["channel_ids"].([]interface{}); ok {
			for _, cid := range cids {
				if cidStr, ok := cid.(string); ok {
					channelIDs = append(channelIDs, cidStr)
				}
			}
		}

		// 模板 ID
		templateID := ""
		if tid, ok := config["template_id"].(string); ok {
			templateID = tid
		}

		// 转换为新格式
		for _, cid := range channelIDs {
			notificationConfigs = append(notificationConfigs, channelTemplateConfig{
				ChannelID:  cid,
				TemplateID: templateID,
			})
		}
	}

	// 是否包含执行结果
	includeExecutionResult := true
	if ier, ok := config["include_execution_result"].(bool); ok {
		includeExecutionResult = ier
	}

	// 是否包含工单信息
	includeIncidentInfo := true
	if iii, ok := config["include_incident_info"].(bool); ok {
		includeIncidentInfo = iii
	}

	// 构建变量 - 包含所有 40 个通知参数，与 notification/variable.go 完全对齐
	variables := make(map[string]interface{})

	// ==================== 基础时间变量 ====================
	now := time.Now()
	variables["timestamp"] = now.Format("2006-01-02 15:04:05")
	variables["date"] = now.Format("2006-01-02")
	variables["time"] = now.Format("15:04:05")

	// ==================== 流程信息 ====================
	variables["flow_instance_id"] = instance.ID
	variables["flow_status"] = instance.Status

	// ==================== system.* (嵌套结构) - 从全局配置读取 ====================
	appCfg := cfg.GetAppConfig()
	variables["system"] = map[string]interface{}{
		"name":    appCfg.Name,
		"url":     appCfg.URL,
		"version": appCfg.Version,
		"env":     appCfg.Env,
	}
	// 同时保留平坦化的兼容变量
	variables["system_name"] = appCfg.Name
	variables["system_version"] = appCfg.Version
	variables["system_env"] = appCfg.Env

	// ==================== 工单信息 (incident.*) ====================
	if includeIncidentInfo && instance.Context != nil {
		if incident, ok := instance.Context["incident"].(map[string]interface{}); ok {
			variables["incident_id"] = incident["id"]
			variables["incident_title"] = incident["title"]
			variables["incident_severity"] = incident["severity"]
			variables["incident_source"] = incident["source_plugin_name"]
			variables["incident_external_id"] = incident["external_id"]
			variables["incident_status"] = incident["status"]
			// 添加所有其他工单字段
			for k, v := range incident {
				variables["incident_"+k] = v
			}
		}
	}

	// ==================== execution.* (嵌套结构) ====================
	executionMap := map[string]interface{}{
		"run_id":           instance.ID.String(),
		"status":           "",
		"status_emoji":     "❓",
		"exit_code":        "",
		"triggered_by":     "workflow",
		"trigger_type":     "workflow",
		"started_at":       "",
		"completed_at":     "",
		"duration":         "",
		"duration_seconds": 0,
		"stdout":           "",
		"stderr":           "",
	}

	if includeExecutionResult && instance.Context != nil {
		if result, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			// 填充 execution 嵌套对象
			executionMap["status"] = result["status"]
			executionMap["exit_code"] = result["exit_code"]
			executionMap["stdout"] = result["stdout"]
			executionMap["stderr"] = result["stderr"]

			// 计算执行时长
			if durationMs, ok := result["duration_ms"].(int64); ok {
				executionMap["duration_seconds"] = int(durationMs / 1000)
				if durationMs < 60000 {
					executionMap["duration"] = fmt.Sprintf("%ds", durationMs/1000)
				} else {
					minutes := durationMs / 60000
					seconds := (durationMs % 60000) / 1000
					executionMap["duration"] = fmt.Sprintf("%dm %ds", minutes, seconds)
				}
			} else if durationMs, ok := result["duration_ms"].(float64); ok {
				// JSONB 反序列化后数字类型为 float64（重试场景）
				ms := int64(durationMs)
				executionMap["duration_seconds"] = int(ms / 1000)
				if ms < 60000 {
					executionMap["duration"] = fmt.Sprintf("%ds", ms/1000)
				} else {
					minutes := ms / 60000
					seconds := (ms % 60000) / 1000
					executionMap["duration"] = fmt.Sprintf("%dm %ds", minutes, seconds)
				}
			} else if durationMs, ok := result["duration_ms"].(int); ok {
				executionMap["duration_seconds"] = durationMs / 1000
				if durationMs < 60000 {
					executionMap["duration"] = fmt.Sprintf("%ds", durationMs/1000)
				} else {
					minutes := durationMs / 60000
					seconds := (durationMs % 60000) / 1000
					executionMap["duration"] = fmt.Sprintf("%dm %ds", minutes, seconds)
				}
			}

			executionMap["started_at"] = result["started_at"]
			executionMap["completed_at"] = result["finished_at"]

			// 状态 emoji
			status := fmt.Sprintf("%v", result["status"])
			switch status {
			case "completed", "success":
				executionMap["status_emoji"] = "✅"
			case "failed":
				executionMap["status_emoji"] = "❌"
			case "timeout":
				executionMap["status_emoji"] = "⏱️"
			case "cancelled":
				executionMap["status_emoji"] = "🚫"
			case "running":
				executionMap["status_emoji"] = "🔄"
			default:
				executionMap["status_emoji"] = "❓"
			}

			// 同时保留平坦化的兼容变量
			variables["execution_status"] = result["status"]
			variables["execution_message"] = result["message"]
			variables["execution_exit_code"] = result["exit_code"]
			variables["execution_stdout"] = result["stdout"]
			variables["execution_stderr"] = result["stderr"]
			variables["execution_duration_ms"] = result["duration_ms"]
			variables["execution_playbook_path"] = result["playbook_path"]
			variables["execution_status_emoji"] = executionMap["status_emoji"]

			// 统计信息 (stats.*)
			if statsRaw, ok := result["stats"]; ok && statsRaw != nil {
				statsMap := map[string]interface{}{
					"ok": 0, "changed": 0, "failed": 0, "unreachable": 0,
					"skipped": 0, "rescued": 0, "ignored": 0, "total": 0, "success_rate": "N/A",
				}

				switch s := statsRaw.(type) {
				case map[string]int:
					statsMap["ok"] = s["ok"]
					statsMap["changed"] = s["changed"]
					statsMap["failed"] = s["failed"]
					statsMap["unreachable"] = s["unreachable"]
					statsMap["skipped"] = s["skipped"]
					statsMap["rescued"] = s["rescued"]
					statsMap["ignored"] = s["ignored"]
					total := s["ok"] + s["changed"] + s["failed"] + s["unreachable"] + s["skipped"]
					statsMap["total"] = total
					if total > 0 {
						successRate := float64(s["ok"]+s["changed"]) / float64(total) * 100
						statsMap["success_rate"] = fmt.Sprintf("%.0f%%", successRate)
					}
				case map[string]interface{}:
					statsMap["ok"] = s["ok"]
					statsMap["changed"] = s["changed"]
					statsMap["failed"] = s["failed"]
					statsMap["unreachable"] = s["unreachable"]
					statsMap["skipped"] = s["skipped"]
					statsMap["rescued"] = s["rescued"]
					statsMap["ignored"] = s["ignored"]
					okCount := toFloat(s["ok"])
					changedCount := toFloat(s["changed"])
					failedCount := toFloat(s["failed"])
					unreachableCount := toFloat(s["unreachable"])
					skippedCount := toFloat(s["skipped"])
					total := okCount + changedCount + failedCount + unreachableCount + skippedCount
					statsMap["total"] = int(total)
					if total > 0 {
						successRate := (okCount + changedCount) / total * 100
						statsMap["success_rate"] = fmt.Sprintf("%.0f%%", successRate)
					}
				}
				variables["stats"] = statsMap
			}

			// 添加其他执行结果字段
			for k, v := range result {
				if _, exists := variables["execution_"+k]; !exists {
					variables["execution_"+k] = v
				}
			}
		}
	}
	variables["execution"] = executionMap

	// ==================== task.* (嵌套结构) ====================
	taskMap := map[string]interface{}{
		"id":            instance.ID.String(),
		"name":          fmt.Sprintf("流程实例 #%s", instance.ID.String()[:8]),
		"target_hosts":  "",
		"host_count":    0,
		"executor_type": "local",
		"is_recurring":  false,
	}
	variables["task"] = taskMap

	// ==================== repository.* (嵌套结构) ====================
	repoMap := map[string]interface{}{
		"id":            "",
		"name":          "",
		"url":           "",
		"branch":        "",
		"playbook":      "",
		"main_playbook": "",
	}
	if instance.Context != nil {
		if execResult, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			if playbookPath, ok := execResult["playbook_path"].(string); ok {
				repoMap["playbook"] = playbookPath
				repoMap["main_playbook"] = playbookPath
			}
		}
	}
	variables["repository"] = repoMap

	// ==================== error.* (嵌套结构) ====================
	errorMap := map[string]interface{}{
		"message": "",
		"host":    "",
	}
	if instance.Context != nil {
		if execResult, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			status := fmt.Sprintf("%v", execResult["status"])
			if status == "failed" || status == "timeout" {
				if stderr, ok := execResult["stderr"].(string); ok && stderr != "" {
					msg := stderr
					if len(msg) > 500 {
						msg = msg[:500] + "..."
					}
					errorMap["message"] = msg
				}
			}
		}
	}
	variables["error"] = errorMap

	// ==================== validation.* (嵌套结构) ====================
	validationMap := map[string]interface{}{
		"total":     0,
		"matched":   0,
		"unmatched": 0,
	}
	if instance.Context != nil {
		if validationSummary, ok := instance.Context["validation_summary"].(map[string]interface{}); ok {
			validationMap["total"] = validationSummary["total"]
			validationMap["matched"] = validationSummary["valid"]
			validationMap["unmatched"] = validationSummary["invalid"]
		}
	}
	variables["validation"] = validationMap

	// ==================== 主机信息 ====================
	hostCount := 0
	if instance.Context != nil {
		if hosts, ok := instance.Context["validated_hosts"]; ok {
			variables["target_hosts"] = hosts
			switch h := hosts.(type) {
			case []interface{}:
				hostCount = len(h)
			case []map[string]interface{}:
				hostCount = len(h)
			case []string:
				hostCount = len(h)
			default:
				hostCount = 1
			}
		}
	}
	variables["host_count"] = hostCount
	taskMap["host_count"] = hostCount

	logger.Exec("NODE").Debug("通知变量: %v", variables)

	// 如果没有配置渠道，记录日志并跳过
	if len(notificationConfigs) == 0 {
		logger.Exec("NODE").Warn("未配置通知渠道，跳过发送")
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelWarn, "未配置通知渠道", map[string]interface{}{
			"config": config,
		})
		return nil
	}

	// 构建通知内容
	subject := fmt.Sprintf("[自愈系统] 流程实例 #%s 执行完成", instance.ID.String()[:8])
	body := fmt.Sprintf("流程实例 #%s 执行完成，状态：%s", instance.ID.String()[:8], instance.Status)

	if executor, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
		if status, ok := executor["status"].(string); ok {
			if status == "completed" {
				subject = fmt.Sprintf("[自愈系统] 流程实例 #%s 执行成功", instance.ID.String()[:8])
			} else {
				subject = fmt.Sprintf("[自愈系统] 流程实例 #%s 执行失败", instance.ID.String()[:8])
			}
		}
		if message, ok := executor["message"].(string); ok {
			body = message
		}
	}

	// 记录日志
	processLogs := []string{
		fmt.Sprintf("共 %d 个通知配置", len(notificationConfigs)),
	}
	for i, cfg := range notificationConfigs {
		processLogs = append(processLogs, fmt.Sprintf("  配置 %d: 渠道=%s, 模板=%s", i+1, cfg.ChannelID, cfg.TemplateID))
	}
	processLogs = append(processLogs, fmt.Sprintf("主题: %s", subject))
	processLogs = append(processLogs, "开始发送通知")

	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "发送通知", map[string]interface{}{
		"input": map[string]interface{}{
			"context":              instance.Context,
			"notification_configs": notificationConfigs,
		},
		"process": processLogs,
	})

	// 使用通知服务发送通知 - 按配置逐个发送
	if e.notificationSvc != nil && len(notificationConfigs) > 0 {
		var allLogs []interface{}
		var lastErr error

		for _, cfg := range notificationConfigs {
			// 解析 channel ID
			channelUUID, err := uuid.Parse(cfg.ChannelID)
			if err != nil {
				logger.Exec("NODE").Error("无效的渠道ID: %s, err: %v", cfg.ChannelID, err)
				continue
			}

			// 解析 template ID
			var templateUUID *uuid.UUID
			if cfg.TemplateID != "" {
				if tid, err := uuid.Parse(cfg.TemplateID); err == nil {
					templateUUID = &tid
				}
			}

			// 调用通知服务发送
			sendReq := notificationSvc.SendNotificationRequest{
				TemplateID: templateUUID,
				ChannelIDs: []uuid.UUID{channelUUID},
				Variables:  variables,
				Subject:    subject,
				Body:       body,
				Format:     "markdown",
			}

			logs, err := e.notificationSvc.Send(ctx, sendReq)
			if err != nil {
				logger.Exec("NODE").Error("通知服务发送失败 (渠道=%s): %v", cfg.ChannelID, err)
				lastErr = err
			} else {
				for _, notifLog := range logs {
					allLogs = append(allLogs, notifLog)
					e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "通知已发送", map[string]interface{}{
						"log_id":      notifLog.ID,
						"status":      notifLog.Status,
						"channel_id":  notifLog.ChannelID,
						"template_id": cfg.TemplateID,
						"subject":     notifLog.Subject,
					})
				}
			}
		}

		if lastErr != nil && len(allLogs) == 0 {
			e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "通知发送失败", map[string]interface{}{
				"error": lastErr.Error(),
			})
		} else {
			logger.Exec("NODE").Info("[%s] 通知发送完成, 共发送 %d 条通知", shortID(instance), len(allLogs))
		}
	} else if len(notificationConfigs) == 0 {
		// 如果没有配置渠道，尝试使用 webhook_url 直接发送
		webhookURL := ""
		if url, ok := config["webhook_url"].(string); ok && url != "" {
			webhookURL = url
		}

		if webhookURL != "" {
			notificationPayload := map[string]interface{}{
				"subject":   subject,
				"body":      body,
				"variables": variables,
				"timestamp": time.Now().Format(time.RFC3339),
			}

			payloadBytes, _ := json.Marshal(notificationPayload)
			// 使用带超时的 HTTP 客户端，避免永久挂起
			httpClient := &http.Client{Timeout: 30 * time.Second}
			resp, err := httpClient.Post(webhookURL, "application/json", bytes.NewReader(payloadBytes))
			if err != nil {
				logger.Exec("NODE").Error("Webhook 通知发送失败: %v", err)
				e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "Webhook 通知发送失败", map[string]interface{}{
					"error":       err.Error(),
					"webhook_url": webhookURL,
				})
			} else {
				respBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				logger.Exec("NODE").Info("Webhook 通知发送成功: HTTP %d, 响应: %s", resp.StatusCode, string(respBody))
				e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "Webhook 通知发送成功", map[string]interface{}{
					"status_code":    resp.StatusCode,
					"webhook_url":    webhookURL,
					"variable_count": len(variables),
				})
			}
		} else {
			logger.Exec("NODE").Warn("未配置通知渠道或 webhook_url，跳过发送")
		}
	}

	return nil
}

// ResumeAfterApproval 审批后恢复执行
func (e *FlowExecutor) ResumeAfterApproval(ctx context.Context, instanceID uuid.UUID, approved bool) error {
	instance, err := e.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return err
	}

	// 辅助函数：更新关联的 Incident 状态
	updateIncidentStatus := func(status string) {
		if instance.IncidentID != nil {
			if incident, err := e.incidentRepo.GetByID(ctx, *instance.IncidentID); err == nil {
				incident.HealingStatus = status
				e.incidentRepo.Update(ctx, incident)
			}
		}
	}

	// 从实例快照解析节点和边
	nodes, edges, err := e.parseFlowSnapshot(instance)
	if err != nil {
		return err
	}

	// 找到当前节点（审批节点）
	var currentNode *model.FlowNode
	for i := range nodes {
		if nodes[i].ID == instance.CurrentNodeID {
			currentNode = &nodes[i]
			break
		}
	}

	if currentNode == nil {
		updateIncidentStatus("failed")
		return e.instanceRepo.UpdateStatus(ctx, instanceID, model.FlowInstanceStatusFailed, "找不到当前节点")
	}

	// 更新状态为运行中
	instance.Status = model.FlowInstanceStatusRunning
	e.instanceRepo.Update(ctx, instance)

	// 根据审批结果选择分支
	var outputHandle string
	if approved {
		outputHandle = "approved"
		logger.Exec("FLOW").Info("[%s] 审批通过，走 approved 分支", instance.ID.String()[:8])
		e.setNodeState(ctx, instance, currentNode.ID, "approved", "")
	} else {
		outputHandle = "rejected"
		logger.Exec("FLOW").Info("[%s] 审批拒绝，走 rejected 分支", instance.ID.String()[:8])
		e.setNodeState(ctx, instance, currentNode.ID, "rejected", "审批被拒绝")
	}

	// 找到对应分支的下一个节点
	nextNode := e.findNextNodeByHandle(nodes, edges, currentNode.ID, outputHandle)
	if nextNode == nil {
		// 如果没有找到对应分支
		if !approved {
			// 拒绝但没有 rejected 分支，流程失败
			updateIncidentStatus("failed")
			return e.instanceRepo.UpdateStatus(ctx, instanceID, model.FlowInstanceStatusFailed, "审批被拒绝")
		}
		// 通过但没有下一个节点，流程完成
		return e.complete(ctx, instance)
	}

	return e.executeNode(ctx, instance, nodes, edges, nextNode)
}

// parseUUID 解析 UUID
func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

// executeCompute 执行计算节点
// 使用表达式引擎计算多个表达式，将结果写入 context
// 配置格式:
//
//	{
//	  "operations": [
//	    {"output_key": "target_ips", "expression": "join(validated_hosts, ',')"},
//	    {"output_key": "host_count", "expression": "len(validated_hosts)"},
//	    {"output_key": "first_host", "expression": "first(validated_hosts).ip_address"}
//	  ]
//	}
func (e *FlowExecutor) executeCompute(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 执行计算节点", shortID(instance))

	config := node.Config

	// 解析 operations 配置
	operations, ok := config["operations"].([]interface{})
	if !ok || len(operations) == 0 {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelWarn, "计算节点配置为空", nil)
		return nil
	}

	// 确保 context 存在
	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}

	// 创建表达式求值器
	evaluator := NewExpressionEvaluator()

	// 记录处理过程
	processLogs := []string{fmt.Sprintf("读取 %d 个计算操作", len(operations))}
	results := make(map[string]interface{})
	var errors []string

	for i, opRaw := range operations {
		op, ok := opRaw.(map[string]interface{})
		if !ok {
			errors = append(errors, fmt.Sprintf("操作 %d: 格式无效", i+1))
			continue
		}

		outputKey, _ := op["output_key"].(string)
		expression, _ := op["expression"].(string)

		if outputKey == "" {
			errors = append(errors, fmt.Sprintf("操作 %d: output_key 为空", i+1))
			continue
		}
		if expression == "" {
			errors = append(errors, fmt.Sprintf("操作 %d: expression 为空", i+1))
			continue
		}

		processLogs = append(processLogs, fmt.Sprintf("计算 %s = %s", outputKey, expression))

		// 执行表达式
		result, err := evaluator.Evaluate(expression, instance.Context)
		if err != nil {
			errMsg := fmt.Sprintf("操作 %d (%s): %v", i+1, outputKey, err)
			errors = append(errors, errMsg)
			logger.Exec("NODE").Warn("[%s] 表达式计算失败: %s", shortID(instance), errMsg)
			continue
		}

		// 写入 context
		instance.Context[outputKey] = result
		results[outputKey] = result
		// 将结果格式化为 JSON 字符串，避免 Go 结构体格式
		resultJSON, _ := json.Marshal(result)
		processLogs = append(processLogs, fmt.Sprintf("  → %s = %s", outputKey, string(resultJSON)))
		logger.Exec("NODE").Debug("[%s] 计算结果: %s = %s", shortID(instance), outputKey, string(resultJSON))
	}

	// 更新实例
	if err := e.instanceRepo.Update(ctx, instance); err != nil {
		logger.Exec("FLOW").Error("更新实例失败: %v", err)
	}

	// 记录日志
	logDetails := map[string]interface{}{
		"input": map[string]interface{}{
			"operations": operations,
			"context":    instance.Context,
		},
		"process": processLogs,
		"output":  results,
	}
	if len(errors) > 0 {
		logDetails["errors"] = errors
	}

	// 判断是否完全失败
	if len(results) == 0 && len(errors) > 0 {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "计算节点执行失败", logDetails)
		return fmt.Errorf("所有计算操作均失败: %v", errors)
	}

	logLevel := model.LogLevelInfo
	message := fmt.Sprintf("计算完成: %d 个变量", len(results))
	if len(errors) > 0 {
		logLevel = model.LogLevelWarn
		message = fmt.Sprintf("计算完成: %d 成功, %d 失败", len(results), len(errors))
	}
	e.logNode(ctx, instance.ID, node.ID, node.Type, logLevel, message, logDetails)

	// 将计算结果写入 node_states
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	if existing, ok := instance.NodeStates[node.ID].(map[string]interface{}); ok {
		existing["computed_results"] = results
		if len(errors) > 0 {
			existing["errors"] = errors
		}
		instance.NodeStates[node.ID] = existing
		e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates)
	}

	logger.Exec("NODE").Info("[%s] 计算节点完成: %d 个变量已写入 context", shortID(instance), len(results))
	return nil
}
