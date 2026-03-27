package httpapi

import (
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/company/auto-healing/internal/notification"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListTemplates 获取模板列表
func (h *NotificationHandler) ListTemplates(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	opts := buildTemplateListOptions(c, page, pageSize)

	templates, total, err := h.svc.ListTemplates(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "NOTIFY", "获取通知模板列表失败", err)
		return
	}
	response.List(c, templates, total, page, pageSize)
}

func buildTemplateListOptions(c *gin.Context, page, pageSize int) *engagementrepo.TemplateListOptions {
	opts := &engagementrepo.TemplateListOptions{
		Page:             page,
		PageSize:         pageSize,
		Name:             GetStringFilter(c, "name"),
		EventType:        c.Query("event_type"),
		Format:           c.Query("format"),
		SupportedChannel: c.Query("supported_channel"),
		SortBy:           c.Query("sort_by"),
		SortOrder:        c.Query("sort_order"),
	}
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		isActive := isActiveStr == "true"
		opts.IsActive = &isActive
	}
	return opts
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
		writeNotificationMutationError(c, err, "模板不存在", "创建通知模板失败")
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
		writeNotificationLookupError(c, err, "模板不存在", "获取通知模板失败")
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
		writeNotificationMutationError(c, err, "模板不存在", "更新通知模板失败")
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
		writeNotificationMutationError(c, err, "模板不存在", "删除通知模板失败")
		return
	}
	response.Message(c, "删除成功")
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
		writeNotificationMutationError(c, err, "模板不存在", "预览通知模板失败")
		return
	}
	response.Success(c, result)
}

// GetAvailableVariables 获取可用变量列表
func (h *NotificationHandler) GetAvailableVariables(c *gin.Context) {
	variables := h.svc.GetAvailableVariables()
	response.Success(c, gin.H{"variables": variables})
}
