package handler

import (
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/notification"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// NotificationHandler 通知处理器
type NotificationHandler struct {
	svc       *notification.Service
	notifRepo *repository.NotificationRepository
}

// NewNotificationHandler 创建通知处理器
func NewNotificationHandler() *NotificationHandler {
	db := database.DB
	return &NotificationHandler{
		svc:       notification.NewService(db, "Auto-Healing", "", "1.0.0"),
		notifRepo: repository.NewNotificationRepository(db),
	}
}

// ==================== 渠道管理 ====================

// ListChannels 获取渠道列表
func (h *NotificationHandler) ListChannels(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	channelType := c.Query("type")
	name := GetStringFilter(c, "name")

	channels, total, err := h.svc.ListChannels(c.Request.Context(), page, pageSize, channelType, name)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, channels, total, page, pageSize)
}

// CreateChannel 创建渠道
func (h *NotificationHandler) CreateChannel(c *gin.Context) {
	var req notification.CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	channel, err := h.svc.CreateChannel(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, channel)
}

// GetChannel 获取渠道详情
func (h *NotificationHandler) GetChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	channel, err := h.svc.GetChannel(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "渠道不存在")
		return
	}

	response.Success(c, channel)
}

// UpdateChannel 更新渠道
func (h *NotificationHandler) UpdateChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	var req notification.UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	channel, err := h.svc.UpdateChannel(c.Request.Context(), id, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, channel)
}

// DeleteChannel 删除渠道
func (h *NotificationHandler) DeleteChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	if err := h.svc.DeleteChannel(c.Request.Context(), id); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Message(c, "删除成功")
}

// TestChannel 测试渠道
func (h *NotificationHandler) TestChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	if err := h.svc.TestChannel(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Message(c, "测试成功")
}

// ==================== 模板管理 ====================

// ListTemplates 获取模板列表
func (h *NotificationHandler) ListTemplates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	opts := &repository.TemplateListOptions{
		Page:             page,
		PageSize:         pageSize,
		Name:             GetStringFilter(c, "name"),
		EventType:        c.Query("event_type"),
		Format:           c.Query("format"),
		SupportedChannel: c.Query("supported_channel"),
		SortBy:           c.Query("sort_by"),
		SortOrder:        c.Query("sort_order"),
	}

	// 处理 is_active 布尔参数
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		isActive := isActiveStr == "true"
		opts.IsActive = &isActive
	}

	templates, total, err := h.svc.ListTemplates(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, templates, total, page, pageSize)
}

// CreateTemplate 创建模板
func (h *NotificationHandler) CreateTemplate(c *gin.Context) {
	var req notification.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	template, err := h.svc.CreateTemplate(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, template)
}

// GetTemplate 获取模板详情
func (h *NotificationHandler) GetTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	template, err := h.svc.GetTemplate(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "模板不存在")
		return
	}

	response.Success(c, template)
}

// UpdateTemplate 更新模板
func (h *NotificationHandler) UpdateTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	var req notification.UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	template, err := h.svc.UpdateTemplate(c.Request.Context(), id, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, template)
}

// DeleteTemplate 删除模板
func (h *NotificationHandler) DeleteTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	if err := h.svc.DeleteTemplate(c.Request.Context(), id); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Message(c, "删除成功")
}

// PreviewTemplateRequest 预览模板请求
type PreviewTemplateRequest struct {
	Variables map[string]interface{} `json:"variables"`
}

// PreviewTemplate 预览模板
func (h *NotificationHandler) PreviewTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	var req PreviewTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.svc.PreviewTemplate(c.Request.Context(), id, req.Variables)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// GetAvailableVariables 获取可用变量列表
func (h *NotificationHandler) GetAvailableVariables(c *gin.Context) {
	variables := h.svc.GetAvailableVariables()
	response.Success(c, gin.H{"variables": variables})
}

// ==================== 通知发送 ====================

// SendNotification 发送通知
func (h *NotificationHandler) SendNotification(c *gin.Context) {
	var req notification.SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	logs, err := h.svc.Send(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 提取 ID 列表
	ids := make([]string, len(logs))
	for i, log := range logs {
		ids[i] = log.ID.String()
	}

	response.Success(c, gin.H{
		"notification_ids": ids,
		"logs":             logs,
	})
}

// ListNotifications 获取通知记录
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	opts := &repository.NotificationLogListOptions{
		Page:        page,
		PageSize:    pageSize,
		Status:      c.Query("status"),
		TaskName:    GetStringFilter(c, "task_name"),
		TriggeredBy: c.Query("triggered_by"),
		Subject:     GetStringFilter(c, "subject"),
		SortBy:      c.Query("sort_by"),
		SortOrder:   c.Query("sort_order"),
	}

	// 解析 UUID 参数
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

	// 解析时间参数
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

	logs, total, err := h.svc.ListNotifications(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, logs, total, page, pageSize)
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
		response.NotFound(c, "通知记录不存在")
		return
	}

	response.Success(c, log)
}

// ==================== 统计 ====================

// GetStats 获取通知统计信息
func (h *NotificationHandler) GetStats(c *gin.Context) {
	stats, err := h.notifRepo.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "获取通知统计信息失败:"+err.Error())
		return
	}
	response.Success(c, stats)
}
