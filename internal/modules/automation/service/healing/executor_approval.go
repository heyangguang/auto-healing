package healing

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

type approvalSettings struct {
	timeoutHours  float64
	timeoutAt     time.Time
	title         string
	description   string
	approvers     model.JSONArray
	approverRoles model.JSONArray
}

func (e *FlowExecutor) executeApproval(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 创建审批任务", shortID(instance))
	settings := parseApprovalSettings(node.Config, instance)
	task := buildApprovalTask(instance.ID, node.ID, settings)
	if err := e.createApprovalTask(ctx, instance, node, task); err != nil {
		return err
	}
	if err := e.markInstanceWaitingApproval(ctx, instance, node.ID, task, settings); err != nil {
		return err
	}
	e.logApprovalWaiting(ctx, instance, node, task, settings)
	logger.Exec("NODE").Info("[%s] 审批任务已创建，ID=%s", shortID(instance), task.ID.String()[:8])
	return nil
}

func parseApprovalSettings(config model.JSON, instance *model.FlowInstance) approvalSettings {
	timeoutHours := 24.0
	if timeout, ok := config["timeout_hours"].(float64); ok {
		timeoutHours = timeout
	}
	settings := approvalSettings{
		timeoutHours: timeoutHours,
		timeoutAt:    time.Now().Add(time.Duration(timeoutHours) * time.Hour),
		title:        fmt.Sprintf("流程实例 %s 审批请求", instance.ID.String()[:8]),
	}
	settings.approvers = approvalJSONArray(config["approvers"])
	settings.approverRoles = approvalJSONArray(config["approver_roles"])
	if title, ok := config["title"].(string); ok && title != "" {
		settings.title = title
	}
	if description, ok := config["description"].(string); ok {
		settings.description = description
	}
	return settings
}

func approvalJSONArray(value interface{}) model.JSONArray {
	if values, ok := value.([]interface{}); ok {
		return values
	}
	if value != nil {
		return model.JSONArray{value}
	}
	return nil
}

func buildApprovalTask(instanceID uuid.UUID, nodeID string, settings approvalSettings) *model.ApprovalTask {
	return &model.ApprovalTask{
		FlowInstanceID: instanceID,
		NodeID:         nodeID,
		Status:         model.ApprovalTaskStatusPending,
		TimeoutAt:      &settings.timeoutAt,
		Approvers:      settings.approvers,
		ApproverRoles:  settings.approverRoles,
	}
}

func (e *FlowExecutor) createApprovalTask(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, task *model.ApprovalTask) error {
	if err := e.approvalRepo.CreateAndEnterWaiting(ctx, task); err != nil {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "创建审批任务失败", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}
	return nil
}

func (e *FlowExecutor) markInstanceWaitingApproval(ctx context.Context, instance *model.FlowInstance, nodeID string, task *model.ApprovalTask, settings approvalSettings) error {
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	now := time.Now().Format(time.RFC3339)
	instance.Status = model.FlowInstanceStatusWaitingApproval
	instance.NodeStates[nodeID] = map[string]interface{}{
		"status":      "waiting_approval",
		"task_id":     task.ID,
		"title":       settings.title,
		"description": settings.description,
		"timeout_at":  settings.timeoutAt.Format(time.RFC3339),
		"created_at":  now,
		"started_at":  now,
	}
	if err := e.ensureWaitingApprovalStatus(ctx, instance.ID); err != nil {
		return err
	}
	return e.persistNodeStates(ctx, instance, "持久化审批等待状态")
}

func (e *FlowExecutor) ensureWaitingApprovalStatus(ctx context.Context, instanceID uuid.UUID) error {
	status, err := e.flowInstanceStatus(ctx, instanceID)
	if err != nil {
		return err
	}
	if status != model.FlowInstanceStatusWaitingApproval {
		return fmt.Errorf("流程实例状态不是 waiting_approval: %s", status)
	}
	return nil
}

func (e *FlowExecutor) logApprovalWaiting(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, task *model.ApprovalTask, settings approvalSettings) {
	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "等待审批", approvalLogDetails(instance, task, settings))
}

func approvalLogDetails(instance *model.FlowInstance, task *model.ApprovalTask, settings approvalSettings) map[string]interface{} {
	return map[string]interface{}{
		"input": map[string]interface{}{
			"context": instance.Context,
		},
		"process": []string{
			fmt.Sprintf("读取配置 title: %s", settings.title),
			fmt.Sprintf("审批人: %v, 审批角色: %v", settings.approvers, settings.approverRoles),
			fmt.Sprintf("超时时间: %.0f 小时", settings.timeoutHours),
			fmt.Sprintf("创建审批任务 ID: %s", task.ID.String()[:8]),
			"流程暂停，等待审批",
		},
		"output": map[string]interface{}{
			"task_id":    task.ID,
			"timeout_at": settings.timeoutAt.Format(time.RFC3339),
			"status":     "waiting_approval",
		},
	}
}

// executeExecution 执行任务节点
// 使用任务模板作为最小执行单位，直接调用 execution.Service
