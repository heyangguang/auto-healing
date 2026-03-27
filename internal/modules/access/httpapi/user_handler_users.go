package httpapi

import (
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListUsers 获取平台管理员用户列表
// 只返回拥有 platform_admin 或 super_admin 角色的用户
func (h *UserHandler) ListUsers(c *gin.Context) {
	sortField := c.Query("sort_by")
	if sortField == "" {
		sortField = c.Query("sort_field")
	}
	params := &accessrepo.UserListParams{
		Page:         getQueryInt(c, "page", 1),
		PageSize:     getQueryInt(c, "page_size", 20),
		Status:       c.Query("status"),
		Username:     GetStringFilter(c, "username"),
		Email:        GetStringFilter(c, "email"),
		DisplayName:  GetStringFilter(c, "display_name"),
		CreatedFrom:  c.Query("created_from"),
		CreatedTo:    c.Query("created_to"),
		SortField:    sortField,
		SortOrder:    c.Query("sort_order"),
		PlatformOnly: true,
	}

	users, total, err := h.userRepo.List(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "获取用户列表失败")
		return
	}
	response.List(c, users, total, params.Page, params.PageSize)
}

// ListSimpleUsers 获取简要用户列表（轻量接口，用于下拉选择）
func (h *UserHandler) ListSimpleUsers(c *gin.Context) {
	users, err := h.userRepo.ListSimple(c.Request.Context(), c.Query("name"), c.DefaultQuery("status", "active"))
	if err != nil {
		response.InternalError(c, "获取用户列表失败")
		return
	}
	response.Success(c, users)
}

// GetUser 获取用户详情
func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, user)
}
