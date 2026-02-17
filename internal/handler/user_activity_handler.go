package handler

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserActivityHandler 用户活动处理器（收藏 + 最近访问）
type UserActivityHandler struct {
	repo *repository.UserActivityRepository
}

// NewUserActivityHandler 创建用户活动处理器
func NewUserActivityHandler() *UserActivityHandler {
	return &UserActivityHandler{
		repo: repository.NewUserActivityRepository(),
	}
}

// favoriteRequest 收藏/最近访问请求体
type favoriteRequest struct {
	MenuKey string `json:"menu_key" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Path    string `json:"path" binding:"required"`
}

// ==================== 收藏 ====================

// ListFavorites 获取当前用户的收藏列表
func (h *UserActivityHandler) ListFavorites(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	favorites, err := h.repo.ListFavorites(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "获取收藏列表失败")
		return
	}

	response.Success(c, favorites)
}

// AddFavorite 添加收藏
func (h *UserActivityHandler) AddFavorite(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req favoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: menu_key, name, path 均为必填")
		return
	}

	fav := &model.UserFavorite{
		UserID:  userID,
		MenuKey: req.MenuKey,
		Name:    req.Name,
		Path:    req.Path,
	}

	if err := h.repo.AddFavorite(c.Request.Context(), fav); err != nil {
		if err.Error() == "该菜单项已收藏" {
			response.Conflict(c, err.Error())
			return
		}
		response.InternalError(c, "添加收藏失败")
		return
	}

	response.Created(c, fav)
}

// RemoveFavorite 取消收藏
func (h *UserActivityHandler) RemoveFavorite(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	menuKey := c.Param("menu_key")
	if menuKey == "" {
		response.BadRequest(c, "menu_key 不能为空")
		return
	}

	if err := h.repo.RemoveFavorite(c.Request.Context(), userID, menuKey); err != nil {
		if err.Error() == "收藏记录不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "取消收藏失败")
		return
	}

	response.Message(c, "取消收藏成功")
}

// ==================== 最近访问 ====================

// ListRecents 获取当前用户的最近访问列表
func (h *UserActivityHandler) ListRecents(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	recents, err := h.repo.ListRecents(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "获取最近访问列表失败")
		return
	}

	response.Success(c, recents)
}

// RecordRecent 记录最近访问
func (h *UserActivityHandler) RecordRecent(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req favoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: menu_key, name, path 均为必填")
		return
	}

	recent := &model.UserRecent{
		UserID:  userID,
		MenuKey: req.MenuKey,
		Name:    req.Name,
		Path:    req.Path,
	}

	if err := h.repo.UpsertRecent(c.Request.Context(), recent); err != nil {
		response.InternalError(c, "记录访问失败")
		return
	}

	response.Success(c, recent)
}
