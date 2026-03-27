package httpapi

import (
	"context"
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	healing "github.com/company/auto-healing/internal/modules/automation/service/healing"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ========== FlowInstance 相关 ==========

// ListInstances 获取流程实例列表
func (h *HealingHandler) ListInstances(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	opts := buildFlowInstanceListOptions(c, page, pageSize)
	instances, total, err := h.instanceRepo.ListSummaryWithOptions(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, "获取流程实例列表失败")
		return
	}

	response.List(c, instances, total, page, pageSize)
}

// GetInstance 获取流程实例详情
func (h *HealingHandler) GetInstance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的实例ID")
		return
	}

	instance, err := h.instanceRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "流程实例不存在")
		return
	}

	response.Success(c, instance)
}

// CancelInstance 取消流程实例
func (h *HealingHandler) CancelInstance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的实例ID")
		return
	}

	// 获取流程实例以获取关联的 IncidentID
	instance, err := h.instanceRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "流程实例不存在")
		return
	}

	updated, err := h.instanceRepo.UpdateStatusIfCurrent(
		c.Request.Context(),
		id,
		[]string{model.FlowInstanceStatusPending, model.FlowInstanceStatusRunning, model.FlowInstanceStatusWaitingApproval},
		model.FlowInstanceStatusCancelled,
		"用户手动取消",
	)
	if err != nil {
		response.InternalError(c, "取消流程实例失败")
		return
	}
	if !updated {
		response.Conflict(c, "当前流程实例状态不允许取消")
		return
	}
	if _, err := h.approvalRepo.CancelPendingByFlowInstance(c.Request.Context(), id, "流程已取消"); err != nil {
		response.InternalError(c, "关闭待审批任务失败")
		return
	}
	h.executor.Cancel(id)
	healing.GetEventBus().PublishFlowComplete(id, false, model.FlowInstanceStatusCancelled, "流程已取消")

	// 更新关联的 Incident 状态为 dismissed（用户主动取消）
	if instance.IncidentID != nil {
		if incident, err := h.incidentRepo.GetByID(c.Request.Context(), *instance.IncidentID); err == nil {
			incident.HealingStatus = "dismissed"
			h.incidentRepo.Update(c.Request.Context(), incident)
		}
	}

	response.Message(c, "流程实例已取消")
}

// RetryInstance 重试流程实例
func (h *HealingHandler) RetryInstance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的实例ID")
		return
	}

	instance, err := h.instanceRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "流程实例不存在")
		return
	}

	// 解析请求体
	var req struct {
		FromNodeID string `json:"from_node_id"` // 可选，从哪个节点开始
	}
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	h.executor.Go(func(rootCtx context.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Exec("RETRY").Error("panic 恢复: %v", r)
			}
		}()
		ctx := rootCtx
		if instance.TenantID != nil {
			ctx = platformrepo.WithTenantID(ctx, *instance.TenantID)
		}
		if err := h.executor.RetryFromNode(ctx, instance, req.FromNodeID); err != nil {
			logger.Exec("RETRY").Error("重试失败: %v", err)
		}
	})

	response.Message(c, "流程实例正在重试")
}

// InstanceEvents 获取流程实例事件流 (SSE)
func (h *HealingHandler) InstanceEvents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的实例ID")
		return
	}

	// 验证实例存在
	instance, err := h.instanceRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "流程实例不存在")
		return
	}

	// 创建 SSE 写入器
	sseWriter, err := NewSSEWriter(c)
	if err != nil {
		response.InternalError(c, "SSE 不支持")
		return
	}

	// 订阅事件
	eventBus := healing.GetEventBus()
	eventCh := eventBus.Subscribe(instance.ID)
	defer eventBus.Unsubscribe(instance.ID, eventCh)

	if writeInitialInstanceEvents(sseWriter, instance) {
		return
	}
	streamInstanceEvents(c.Request.Context(), sseWriter, eventCh)
}

func buildFlowInstanceListOptions(c *gin.Context, page, pageSize int) automationrepo.FlowInstanceListOptions {
	opts := automationrepo.FlowInstanceListOptions{
		Page:           page,
		PageSize:       pageSize,
		Status:         c.Query("status"),
		FlowName:       GetStringFilter(c, "flow_name"),
		RuleName:       GetStringFilter(c, "rule_name"),
		IncidentTitle:  GetStringFilter(c, "incident_title"),
		CurrentNodeID:  c.Query("current_node_id"),
		ErrorMessage:   GetStringFilter(c, "error_message"),
		SortBy:         c.Query("sort_by"),
		SortOrder:      c.Query("sort_order"),
		ApprovalStatus: c.Query("approval_status"),
		CreatedFrom:    parseOptionalTime(c.Query("created_from")),
		CreatedTo:      parseOptionalTime(c.Query("created_to")),
		StartedFrom:    parseOptionalTime(c.Query("started_from")),
		StartedTo:      parseOptionalTime(c.Query("started_to")),
		CompletedFrom:  parseOptionalTime(c.Query("completed_from")),
		CompletedTo:    parseOptionalTime(c.Query("completed_to")),
		MinNodes:       parseOptionalInt(c.Query("min_nodes")),
		MaxNodes:       parseOptionalInt(c.Query("max_nodes")),
		MinFailedNodes: parseOptionalInt(c.Query("min_failed_nodes")),
		MaxFailedNodes: parseOptionalInt(c.Query("max_failed_nodes")),
	}
	applyFlowInstanceUUIDOptions(&opts, c)
	applyFlowInstanceBoolOptions(&opts, c)
	return opts
}

func applyFlowInstanceUUIDOptions(opts *automationrepo.FlowInstanceListOptions, c *gin.Context) {
	opts.FlowID = parseOptionalUUID(c.Query("flow_id"))
	opts.RuleID = parseOptionalUUID(c.Query("rule_id"))
	opts.IncidentID = parseOptionalUUID(c.Query("incident_id"))
}

func applyFlowInstanceBoolOptions(opts *automationrepo.FlowInstanceListOptions, c *gin.Context) {
	if str := c.Query("has_error"); str != "" {
		val := str == "true" || str == "1"
		opts.HasError = &val
	}
}

func parseOptionalUUID(value string) *uuid.UUID {
	if value == "" {
		return nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseOptionalTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &t
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return &t
	}
	return nil
}

func parseOptionalInt(value string) *int {
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func writeInitialInstanceEvents(sseWriter *SSEWriter, instance *model.FlowInstance) bool {
	sseWriter.WriteEvent("connected", map[string]interface{}{
		"instance_id": instance.ID.String(),
		"status":      instance.Status,
	})
	if !isTerminalFlowStatus(instance.Status) {
		return false
	}
	sseWriter.WriteEvent(string(healing.EventFlowComplete), map[string]interface{}{
		"success": instance.Status == model.FlowInstanceStatusCompleted,
		"status":  instance.Status,
		"message": terminalFlowMessage(instance),
	})
	return true
}

func streamInstanceEvents(ctx context.Context, sseWriter *SSEWriter, eventCh <-chan healing.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			sseWriter.WriteEvent(string(event.Type), event.Data)
			if event.Type == healing.EventFlowComplete {
				return
			}
		}
	}
}

func isTerminalFlowStatus(status string) bool {
	return status == model.FlowInstanceStatusCompleted ||
		status == model.FlowInstanceStatusFailed ||
		status == model.FlowInstanceStatusCancelled
}

func terminalFlowMessage(instance *model.FlowInstance) string {
	if instance.ErrorMessage != "" {
		return instance.ErrorMessage
	}
	return "流程已结束"
}
