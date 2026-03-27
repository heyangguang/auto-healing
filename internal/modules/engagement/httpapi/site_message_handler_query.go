package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	platformevents "github.com/company/auto-healing/internal/platform/events"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListMessages 分页查询站内信列表
func (h *SiteMessageHandler) ListMessages(c *gin.Context) {
	userID, ok := parseCurrentUserID(c)
	if !ok {
		return
	}
	page, pageSize := parsePagination(c, 10)
	tenantID, userCreatedAt, ok := h.currentTenantContext(c, userID)
	if !ok {
		return
	}
	messages, total, err := h.repo.List(
		c.Request.Context(),
		userID,
		&tenantID,
		userCreatedAt,
		page,
		pageSize,
		c.Query("keyword"),
		c.Query("category"),
		c.Query("is_read"),
		c.Query("date_from"),
		c.Query("date_to"),
		c.Query("sort"),
		c.Query("order"),
	)
	if err != nil {
		response.InternalError(c, "查询站内信失败")
		return
	}
	response.List(c, messages, total, page, pageSize)
}

// GetUnreadCount 获取未读消息数量
func (h *SiteMessageHandler) GetUnreadCount(c *gin.Context) {
	userID, ok := parseCurrentUserID(c)
	if !ok {
		return
	}
	tenantID, userCreatedAt, ok := h.currentTenantContext(c, userID)
	if !ok {
		return
	}
	count, err := h.repo.GetUnreadCount(c.Request.Context(), userID, &tenantID, userCreatedAt)
	if err != nil {
		response.InternalError(c, "获取未读数量失败")
		return
	}
	response.Success(c, gin.H{"unread_count": count})
}

// MarkRead 批量标记已读
func (h *SiteMessageHandler) MarkRead(c *gin.Context) {
	userID, ok := parseCurrentUserID(c)
	if !ok {
		return
	}

	var req markReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：ids 为必填数组")
		return
	}
	if len(req.IDs) == 0 {
		response.BadRequest(c, "ids 不能为空")
		return
	}

	messageIDs, err := parseUUIDList(req.IDs)
	if err != nil {
		response.BadRequest(c, "无效的消息 ID")
		return
	}
	if err := h.repo.MarkRead(c.Request.Context(), userID, messageIDs); err != nil {
		response.InternalError(c, "标记已读失败")
		return
	}
	response.Message(c, "标记已读成功")
}

// MarkAllRead 全部标记已读
func (h *SiteMessageHandler) MarkAllRead(c *gin.Context) {
	userID, ok := parseCurrentUserID(c)
	if !ok {
		return
	}
	tenantID, userCreatedAt, ok := h.currentTenantContext(c, userID)
	if !ok {
		return
	}
	count, err := h.repo.MarkAllRead(c.Request.Context(), userID, &tenantID, userCreatedAt)
	if err != nil {
		response.InternalError(c, "全部标记已读失败")
		return
	}
	response.Success(c, gin.H{"marked_count": count})
}

// CreateMessage 创建站内信（管理员）
func (h *SiteMessageHandler) CreateMessage(c *gin.Context) {
	var req createSiteMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：category, title, content 均为必填")
		return
	}
	if !validSiteMessageCategory(req.Category) {
		response.BadRequest(c, "无效的消息分类: "+req.Category)
		return
	}
	if len(req.TargetTenantIDs) == 0 {
		h.createBroadcastMessage(c, req)
		return
	}
	h.createTenantScopedMessages(c, req)
}

func (h *SiteMessageHandler) createBroadcastMessage(c *gin.Context, req createSiteMessageRequest) {
	msg := &model.SiteMessage{Category: req.Category, Title: req.Title, Content: req.Content}
	if err := h.repo.Create(c.Request.Context(), msg); err != nil {
		response.InternalError(c, "创建站内信失败")
		return
	}
	h.eventBus.Broadcast()
	response.Created(c, msg)
}

func (h *SiteMessageHandler) createTenantScopedMessages(c *gin.Context, req createSiteMessageRequest) {
	tenantIDs, err := parseUUIDList(req.TargetTenantIDs)
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}
	for _, tenantID := range tenantIDs {
		if _, err := h.tenantRepo.GetByID(c.Request.Context(), tenantID); err != nil {
			response.BadRequest(c, "租户不存在: "+tenantID.String())
			return
		}
	}

	messages := make([]*model.SiteMessage, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tid := tenantID
		messages = append(messages, &model.SiteMessage{
			Category:       req.Category,
			Title:          req.Title,
			Content:        req.Content,
			TargetTenantID: &tid,
		})
	}
	if err := h.repo.CreateBatch(c.Request.Context(), messages); err != nil {
		response.InternalError(c, "批量创建站内信失败")
		return
	}
	h.eventBus.Broadcast()
	response.Success(c, gin.H{"created_count": len(messages)})
}

// GetCategories 获取消息分类枚举列表
func (h *SiteMessageHandler) GetCategories(c *gin.Context) {
	response.Success(c, model.AllSiteMessageCategories)
}

// GetSettings 获取站内信设置（代理到 platform_settings）
func (h *SiteMessageHandler) GetSettings(c *gin.Context) {
	retentionDays := h.platformSettings.GetIntValue(c.Request.Context(), "site_message.retention_days", 90)
	setting, _ := h.platformSettings.GetByKey(c.Request.Context(), "site_message.retention_days")
	updatedAt := ""
	if setting != nil {
		updatedAt = setting.UpdatedAt.Format("2006-01-02T15:04:05.000000-07:00")
	}
	response.Success(c, siteMessageSettingsResponse{RetentionDays: retentionDays, UpdatedAt: updatedAt})
}

// UpdateSettings 更新站内信设置（代理到 platform_settings）
func (h *SiteMessageHandler) UpdateSettings(c *gin.Context) {
	var req updateSiteMessageSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：retention_days 必须为 1-3650 之间的整数")
		return
	}

	var updatedBy *uuid.UUID
	if userIDStr := middleware.GetUserID(c); userIDStr != "" {
		if uid, parseErr := uuid.Parse(userIDStr); parseErr == nil {
			updatedBy = &uid
		}
	}
	updated, err := h.platformSettings.Update(c.Request.Context(), "site_message.retention_days", strconv.Itoa(req.RetentionDays), updatedBy)
	if err != nil {
		response.InternalError(c, "更新设置失败")
		return
	}
	response.Success(c, siteMessageSettingsResponse{
		RetentionDays: req.RetentionDays,
		UpdatedAt:     updated.UpdatedAt.Format("2006-01-02T15:04:05.000000-07:00"),
	})
}

// Events SSE 端点 — 实时推送站内消息通知
func (h *SiteMessageHandler) Events(c *gin.Context) {
	userID, ok := parseCurrentUserID(c)
	if !ok {
		return
	}
	flusher, ok := writeSiteMessageSSEHeaders(c)
	if !ok {
		return
	}

	ch := h.eventBus.Subscribe(userID)
	defer h.eventBus.Unsubscribe(userID, ch)
	logger.Info("SSE 站内消息连接建立: userID=%s, 当前在线=%d", userID, h.eventBus.GetOnlineCount())
	h.pushInitialUnreadCount(c, flusher, userID)
	h.streamSiteMessageEvents(c, flusher, userID, ch)
}

func (h *SiteMessageHandler) pushInitialUnreadCount(c *gin.Context, flusher http.Flusher, userID uuid.UUID) {
	tenantID, userCreatedAt, ok := h.currentTenantContext(c, userID)
	if !ok {
		return
	}
	if count, err := h.repo.GetUnreadCount(c.Request.Context(), userID, &tenantID, userCreatedAt); err == nil {
		fmt.Fprintf(c.Writer, "event: init\ndata: %s\n\n", siteMessageEventData("init", count))
		flusher.Flush()
	}
}

func (h *SiteMessageHandler) streamSiteMessageEvents(c *gin.Context, flusher http.Flusher, userID uuid.UUID, ch chan platformevents.MessageEvent) {
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			logger.Info("SSE 站内消息连接断开: userID=%s, 剩余在线=%d", userID, h.eventBus.GetOnlineCount()-1)
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, siteMessageEventData(event.Type, 0))
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
