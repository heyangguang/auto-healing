package httpapi

import (
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SendNotification 发送通知
func (h *NotificationHandler) SendNotification(c *gin.Context) {
	var req notification.SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	logs, err := h.svc.Send(c.Request.Context(), req)
	if err != nil {
		writeNotificationSendError(c, err, logs)
		return
	}
	response.Success(c, gin.H{
		"notification_ids": notificationLogIDs(logs),
		"logs":             logs,
	})
}

func notificationLogIDs(logs []*model.NotificationLog) []string {
	ids := make([]string, len(logs))
	for i, log := range logs {
		ids[i] = log.ID.String()
	}
	return ids
}

// ListNotifications 获取通知记录
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	opts := buildNotificationLogListOptions(c, page, pageSize)

	logs, total, err := h.svc.ListNotifications(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "NOTIFY", "获取通知记录列表失败", err)
		return
	}
	response.List(c, logs, total, page, pageSize)
}

func buildNotificationLogListOptions(c *gin.Context, page, pageSize int) *engagementrepo.NotificationLogListOptions {
	opts := &engagementrepo.NotificationLogListOptions{
		Page:        page,
		PageSize:    pageSize,
		Status:      c.Query("status"),
		TaskName:    GetStringFilter(c, "task_name"),
		TriggeredBy: c.Query("triggered_by"),
		Subject:     GetStringFilter(c, "subject"),
		SortBy:      c.Query("sort_by"),
		SortOrder:   c.Query("sort_order"),
	}
	parseOptionalNotificationUUIDs(c, opts)
	parseOptionalNotificationDates(c, opts)
	return opts
}

func parseOptionalNotificationUUIDs(c *gin.Context, opts *engagementrepo.NotificationLogListOptions) {
	if cidStr := c.Query("channel_id"); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			opts.ChannelID = &cid
		}
	}
	if tidStr := c.Query("template_id"); tidStr != "" {
		if tid, err := uuid.Parse(tidStr); err == nil {
			opts.TemplateID = &tid
		}
	}
	if taskIDStr := c.Query("task_id"); taskIDStr != "" {
		if taskID, err := uuid.Parse(taskIDStr); err == nil {
			opts.TaskID = &taskID
		}
	}
	if runIDStr := c.Query("execution_run_id"); runIDStr != "" {
		if runID, err := uuid.Parse(runIDStr); err == nil {
			opts.ExecutionRunID = &runID
		}
	}
}

func parseOptionalNotificationDates(c *gin.Context, opts *engagementrepo.NotificationLogListOptions) {
	if afterStr := c.Query("created_after"); afterStr != "" {
		if after, err := time.Parse(time.RFC3339, afterStr); err == nil {
			opts.CreatedAfter = &after
		}
	}
	if beforeStr := c.Query("created_before"); beforeStr != "" {
		if before, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			opts.CreatedBefore = &before
		}
	}
}

// GetNotification 获取通知详情
func (h *NotificationHandler) GetNotification(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	log, err := h.svc.GetNotification(c.Request.Context(), id)
	if err != nil {
		writeNotificationLookupError(c, err, "通知记录不存在", "获取通知记录失败")
		return
	}
	response.Success(c, log)
}

// GetStats 获取通知统计信息
func (h *NotificationHandler) GetStats(c *gin.Context) {
	stats, err := h.notifRepo.GetStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "NOTIFY", "获取通知统计信息失败", err)
		return
	}
	response.Success(c, stats)
}
