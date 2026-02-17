package handler

import (
	"strconv"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SiteMessageHandler 站内信处理器
type SiteMessageHandler struct {
	repo *repository.SiteMessageRepository
}

// NewSiteMessageHandler 创建站内信处理器
func NewSiteMessageHandler() *SiteMessageHandler {
	return &SiteMessageHandler{
		repo: repository.NewSiteMessageRepository(),
	}
}

// ==================== DTO ====================

// createSiteMessageRequest 创建站内信请求
type createSiteMessageRequest struct {
	Category string `json:"category" binding:"required"`
	Title    string `json:"title" binding:"required"`
	Content  string `json:"content" binding:"required"`
}

// markReadRequest 批量标记已读请求
type markReadRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

// updateSettingsRequest 更新设置请求
type updateSettingsRequest struct {
	RetentionDays int `json:"retention_days" binding:"required,min=1,max=3650"`
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

	messages, total, err := h.repo.List(c.Request.Context(), userID, page, pageSize, keyword, category)
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

	count, err := h.repo.GetUnreadCount(c.Request.Context(), userID)
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

	count, err := h.repo.MarkAllRead(c.Request.Context(), userID)
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

	msg := &model.SiteMessage{
		Category: req.Category,
		Title:    req.Title,
		Content:  req.Content,
	}

	if err := h.repo.Create(c.Request.Context(), msg); err != nil {
		response.InternalError(c, "创建站内信失败")
		return
	}

	response.Created(c, msg)
}

// ==================== 分类枚举 ====================

// GetCategories 获取消息分类枚举列表
// GET /api/v1/site-messages/categories
func (h *SiteMessageHandler) GetCategories(c *gin.Context) {
	response.Success(c, model.AllSiteMessageCategories)
}

// ==================== 设置 ====================

// GetSettings 获取站内信设置
// GET /api/v1/site-messages/settings
func (h *SiteMessageHandler) GetSettings(c *gin.Context) {
	settings, err := h.repo.GetSettings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "获取设置失败")
		return
	}
	response.Success(c, settings)
}

// UpdateSettings 更新站内信设置
// PUT /api/v1/site-messages/settings
func (h *SiteMessageHandler) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：retention_days 必须为 1-3650 之间的整数")
		return
	}

	settings, err := h.repo.UpdateSettings(c.Request.Context(), req.RetentionDays)
	if err != nil {
		response.InternalError(c, "更新设置失败")
		return
	}

	response.Success(c, settings)
}
