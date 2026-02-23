package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SiteMessageHandler 站内信处理器
type SiteMessageHandler struct {
	repo             *repository.SiteMessageRepository
	platformSettings *repository.PlatformSettingsRepository
	eventBus         *MessageEventBus
	tenantRepo       *repository.TenantRepository
	userRepo         *repository.UserRepository
}

// NewSiteMessageHandler 创建站内信处理器
func NewSiteMessageHandler() *SiteMessageHandler {
	return &SiteMessageHandler{
		repo:             repository.NewSiteMessageRepository(),
		platformSettings: repository.NewPlatformSettingsRepository(),
		eventBus:         GetMessageEventBus(),
		tenantRepo:       repository.NewTenantRepository(),
		userRepo:         repository.NewUserRepository(),
	}
}

// ==================== DTO ====================

// createSiteMessageRequest 创建站内信请求
type createSiteMessageRequest struct {
	Category        string   `json:"category" binding:"required"`
	Title           string   `json:"title" binding:"required"`
	Content         string   `json:"content" binding:"required"`
	TargetTenantIDs []string `json:"target_tenant_ids"` // 可选，为空=全局广播
}

// markReadRequest 批量标记已读请求
type markReadRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

// updateSiteMessageSettingsRequest 更新站内信设置请求（代理到 platform_settings）
type updateSiteMessageSettingsRequest struct {
	RetentionDays int `json:"retention_days" binding:"required,min=1,max=3650"`
}

// siteMessageSettingsResponse 站内信设置响应（兼容旧格式）
type siteMessageSettingsResponse struct {
	RetentionDays int    `json:"retention_days"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// ==================== 辅助方法 ====================

// getUserCreatedAt 获取用户创建时间（用于过滤站内信：不显示用户注册之前的消息）
func (h *SiteMessageHandler) getUserCreatedAt(c *gin.Context, userID uuid.UUID) time.Time {
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		logger.Warn("获取用户创建时间失败: userID=%s, err=%v", userID, err)
		return time.Time{} // 返回零值，不做过滤
	}
	return user.CreatedAt
}

// ==================== 消息查询 ====================

// ListMessages 分页查询站内信列表
// GET /api/v1/site-messages?page=1&page_size=10&keyword=xxx&category=xxx
func (h *SiteMessageHandler) ListMessages(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	keyword := c.Query("keyword")
	category := c.Query("category")
	isRead := c.Query("is_read")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")
	sortField := c.Query("sort")
	order := c.Query("order")

	tenantID := middleware.GetTenantUUID(c)
	userCreatedAt := h.getUserCreatedAt(c, userID)
	messages, total, err := h.repo.List(c.Request.Context(), userID, &tenantID, userCreatedAt, page, pageSize, keyword, category, isRead, dateFrom, dateTo, sortField, order)
	if err != nil {
		response.InternalError(c, "查询站内信失败")
		return
	}

	response.List(c, messages, total, page, pageSize)
}

// GetUnreadCount 获取未读消息数量
// GET /api/v1/site-messages/unread-count
func (h *SiteMessageHandler) GetUnreadCount(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	tenantID := middleware.GetTenantUUID(c)
	userCreatedAt := h.getUserCreatedAt(c, userID)
	count, err := h.repo.GetUnreadCount(c.Request.Context(), userID, &tenantID, userCreatedAt)
	if err != nil {
		response.InternalError(c, "获取未读数量失败")
		return
	}

	response.Success(c, gin.H{"unread_count": count})
}

// ==================== 标记已读 ====================

// MarkRead 批量标记已读
// PUT /api/v1/site-messages/read
func (h *SiteMessageHandler) MarkRead(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
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

	// 解析 UUID 列表
	messageIDs := make([]uuid.UUID, 0, len(req.IDs))
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			response.BadRequest(c, "无效的消息 ID: "+idStr)
			return
		}
		messageIDs = append(messageIDs, id)
	}

	if err := h.repo.MarkRead(c.Request.Context(), userID, messageIDs); err != nil {
		response.InternalError(c, "标记已读失败")
		return
	}

	response.Message(c, "标记已读成功")
}

// MarkAllRead 全部标记已读
// PUT /api/v1/site-messages/read-all
func (h *SiteMessageHandler) MarkAllRead(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	tenantID := middleware.GetTenantUUID(c)
	userCreatedAt := h.getUserCreatedAt(c, userID)
	count, err := h.repo.MarkAllRead(c.Request.Context(), userID, &tenantID, userCreatedAt)
	if err != nil {
		response.InternalError(c, "全部标记已读失败")
		return
	}

	response.Success(c, gin.H{"marked_count": count})
}

// ==================== 创建消息 ====================

// CreateMessage 创建站内信（管理员）
// POST /api/v1/site-messages
func (h *SiteMessageHandler) CreateMessage(c *gin.Context) {
	var req createSiteMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：category, title, content 均为必填")
		return
	}

	// 验证分类是否合法
	validCategory := false
	for _, cat := range model.AllSiteMessageCategories {
		if cat.Value == req.Category {
			validCategory = true
			break
		}
	}
	if !validCategory {
		response.BadRequest(c, "无效的消息分类: "+req.Category)
		return
	}

	// 如果没有指定目标租户，则为全局广播（target_tenant_id = NULL）
	if len(req.TargetTenantIDs) == 0 {
		msg := &model.SiteMessage{
			Category: req.Category,
			Title:    req.Title,
			Content:  req.Content,
		}

		if err := h.repo.Create(c.Request.Context(), msg); err != nil {
			response.InternalError(c, "创建站内信失败")
			return
		}

		// 广播通知所有在线用户有新消息
		h.eventBus.Broadcast()

		response.Created(c, msg)
		return
	}

	// 指定了目标租户：为每个 tenant_id 创建一条消息
	// 1. 解析并验证每个 tenant_id
	tenantIDs := make([]uuid.UUID, 0, len(req.TargetTenantIDs))
	for _, idStr := range req.TargetTenantIDs {
		tid, err := uuid.Parse(idStr)
		if err != nil {
			response.BadRequest(c, "无效的租户 ID: "+idStr)
			return
		}
		tenantIDs = append(tenantIDs, tid)
	}

	// 2. 验证每个租户是否真实存在
	for _, tid := range tenantIDs {
		if _, err := h.tenantRepo.GetByID(c.Request.Context(), tid); err != nil {
			response.BadRequest(c, "租户不存在: "+tid.String())
			return
		}
	}

	// 3. 为每个租户创建一条消息（同一事务）
	msgs := make([]*model.SiteMessage, 0, len(tenantIDs))
	for _, tid := range tenantIDs {
		tidCopy := tid // 避免闭包捕获循环变量
		msgs = append(msgs, &model.SiteMessage{
			Category:       req.Category,
			Title:          req.Title,
			Content:        req.Content,
			TargetTenantID: &tidCopy,
		})
	}

	if err := h.repo.CreateBatch(c.Request.Context(), msgs); err != nil {
		response.InternalError(c, "批量创建站内信失败")
		return
	}

	// 广播通知所有在线用户有新消息
	h.eventBus.Broadcast()

	response.Success(c, gin.H{"created_count": len(msgs)})
}

// ==================== 分类枚举 ====================

// GetCategories 获取消息分类枚举列表
// GET /api/v1/site-messages/categories
func (h *SiteMessageHandler) GetCategories(c *gin.Context) {
	response.Success(c, model.AllSiteMessageCategories)
}

// ==================== 设置（代理到 platform_settings） ====================

// GetSettings 获取站内信设置（代理到 platform_settings）
// GET /api/v1/site-messages/settings
func (h *SiteMessageHandler) GetSettings(c *gin.Context) {
	retentionDays := h.platformSettings.GetIntValue(c.Request.Context(), "site_message.retention_days", 90)

	setting, _ := h.platformSettings.GetByKey(c.Request.Context(), "site_message.retention_days")
	updatedAt := ""
	if setting != nil {
		updatedAt = setting.UpdatedAt.Format("2006-01-02T15:04:05.000000-07:00")
	}

	response.Success(c, siteMessageSettingsResponse{
		RetentionDays: retentionDays,
		UpdatedAt:     updatedAt,
	})
}

// UpdateSettings 更新站内信设置（代理到 platform_settings）
// PUT /api/v1/site-messages/settings
func (h *SiteMessageHandler) UpdateSettings(c *gin.Context) {
	var req updateSiteMessageSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：retention_days 必须为 1-3650 之间的整数")
		return
	}

	// 获取操作人
	var updatedBy *uuid.UUID
	if userIDStr := middleware.GetUserID(c); userIDStr != "" {
		if uid, parseErr := uuid.Parse(userIDStr); parseErr == nil {
			updatedBy = &uid
		}
	}

	updated, err := h.platformSettings.Update(
		c.Request.Context(),
		"site_message.retention_days",
		strconv.Itoa(req.RetentionDays),
		updatedBy,
	)
	if err != nil {
		response.InternalError(c, "更新设置失败")
		return
	}

	response.Success(c, siteMessageSettingsResponse{
		RetentionDays: req.RetentionDays,
		UpdatedAt:     updated.UpdatedAt.Format("2006-01-02T15:04:05.000000-07:00"),
	})
}

// ==================== SSE 实时推送 ====================

// Events SSE 端点 — 实时推送站内消息通知
// GET /api/v1/site-messages/events?token=xxx
// 前端通过 EventSource 连接此端点，收到 new_message 事件后自行调 unread-count 刷新角标
func (h *SiteMessageHandler) Events(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no") // nginx 不缓冲

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.InternalError(c, "SSE 不支持")
		return
	}

	// 订阅事件总线
	ch := h.eventBus.Subscribe(userID)
	defer h.eventBus.Unsubscribe(userID, ch)

	logger.Info("SSE 站内消息连接建立: userID=%s, 当前在线=%d", userID, h.eventBus.GetOnlineCount())

	// 连接建立后立即推送当前未读数量
	tenantID := middleware.GetTenantUUID(c)
	userCreatedAt := h.getUserCreatedAt(c, userID)
	if count, err := h.repo.GetUnreadCount(c.Request.Context(), userID, &tenantID, userCreatedAt); err == nil {
		data := fmt.Sprintf(`{"type":"init","unread_count":%d}`, count)
		fmt.Fprintf(c.Writer, "event: init\ndata: %s\n\n", data)
		flusher.Flush()
	}

	// 心跳定时器
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			// 客户端断开连接
			logger.Info("SSE 站内消息连接断开: userID=%s, 剩余在线=%d", userID, h.eventBus.GetOnlineCount()-1)
			return
		case event, ok := <-ch:
			if !ok {
				// 通道已关闭
				return
			}
			// 推送新消息通知
			data := fmt.Sprintf(`{"type":"%s"}`, event.Type)
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		case <-heartbeat.C:
			// 心跳保活
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
