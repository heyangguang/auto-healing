package handler

import (
	"fmt"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserHandler 用户管理处理器
type UserHandler struct {
	userRepo *repository.UserRepository
	roleRepo *repository.RoleRepository
	authSvc  *authService.Service
}

// NewUserHandler 创建用户处理器
func NewUserHandler(authSvc *authService.Service) *UserHandler {
	return &UserHandler{
		userRepo: repository.NewUserRepository(),
		roleRepo: repository.NewRoleRepository(),
		authSvc:  authSvc,
	}
}

// ListUsers 获取用户列表
func (h *UserHandler) ListUsers(c *gin.Context) {
	params := &repository.UserListParams{
		Page:        getQueryInt(c, "page", 1),
		PageSize:    getQueryInt(c, "page_size", 20),
		Status:      c.Query("status"),
		Search:      c.Query("search"),
		Username:    c.Query("username"),
		Email:       c.Query("email"),
		DisplayName: c.Query("display_name"),
		RoleID:      c.Query("role_id"),
		CreatedFrom: c.Query("created_from"),
		CreatedTo:   c.Query("created_to"),
		SortField:   c.Query("sort_field"),
		SortOrder:   c.Query("sort_order"),
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
	search := c.Query("search")
	status := c.DefaultQuery("status", "active") // 默认只返回活跃用户

	users, err := h.userRepo.ListSimple(c.Request.Context(), search, status)
	if err != nil {
		response.InternalError(c, "获取用户列表失败")
		return
	}

	response.Success(c, users)
}

// CreateUser 创建用户
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req authService.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), &req)
	if err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	response.Created(c, user)
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

// UpdateUser 更新用户
func (h *UserHandler) UpdateUser(c *gin.Context) {
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

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Status != "" {
		user.Status = req.Status
	}

	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		response.InternalError(c, "更新失败")
		return
	}

	response.Success(c, user)
}

// DeleteUser 删除用户
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	currentUserID := middleware.GetUserID(c)
	if currentUserID == id.String() {
		response.BadRequest(c, "不能删除自己的账户")
		return
	}

	if err := h.userRepo.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "删除失败")
		return
	}

	response.Message(c, "删除成功")
}

// ResetPassword 重置密码
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := h.authSvc.ResetPassword(c.Request.Context(), id, req.NewPassword); err != nil {
		response.InternalError(c, "重置密码失败")
		return
	}

	response.Message(c, "密码重置成功")
}

// AssignUserRoles 分配用户角色
func (h *UserHandler) AssignUserRoles(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req AssignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := h.userRepo.AssignRoles(c.Request.Context(), id, req.RoleIDs); err != nil {
		response.InternalError(c, "分配角色失败")
		return
	}

	user, _ := h.userRepo.GetByID(c.Request.Context(), id)
	response.Success(c, user)
}

// RoleHandler 角色管理处理器
type RoleHandler struct {
	roleRepo *repository.RoleRepository
	permRepo *repository.PermissionRepository
}

// NewRoleHandler 创建角色处理器
func NewRoleHandler() *RoleHandler {
	return &RoleHandler{
		roleRepo: repository.NewRoleRepository(),
		permRepo: repository.NewPermissionRepository(),
	}
}

// ListRoles 获取角色列表（含统计信息）
func (h *RoleHandler) ListRoles(c *gin.Context) {
	filter := repository.RoleFilter{
		Search: c.Query("search"),
	}

	roles, err := h.roleRepo.ListWithFilter(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "获取角色列表失败")
		return
	}

	stats, _ := h.roleRepo.GetRoleStats(c.Request.Context())

	type RoleWithStats struct {
		model.Role
		UserCount       int64 `json:"user_count"`
		PermissionCount int64 `json:"permission_count"`
	}

	result := make([]RoleWithStats, len(roles))
	for i, role := range roles {
		rws := RoleWithStats{
			Role:            role,
			PermissionCount: int64(len(role.Permissions)),
		}
		if s, ok := stats[role.ID.String()]; ok {
			rws.UserCount = s.UserCount
		}
		result[i] = rws
	}

	response.Success(c, result)
}

// CreateRole 创建角色
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := h.roleRepo.Create(c.Request.Context(), &role); err != nil {
		response.InternalError(c, "创建角色失败")
		return
	}

	response.Created(c, role)
}

// GetRole 获取角色详情
func (h *RoleHandler) GetRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色ID")
		return
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "角色不存在")
		return
	}

	response.Success(c, role)
}

// UpdateRole 更新角色
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色ID")
		return
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "角色不存在")
		return
	}

	// 系统角色不允许修改
	if role.IsSystem {
		response.BadRequest(c, "系统内置角色不允许修改")
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if req.DisplayName != "" {
		role.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		role.Description = req.Description
	}

	if err := h.roleRepo.Update(c.Request.Context(), role); err != nil {
		response.InternalError(c, "更新失败")
		return
	}

	response.Success(c, role)
}

// DeleteRole 删除角色
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色ID")
		return
	}

	if err := h.roleRepo.Delete(c.Request.Context(), id); err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	response.Message(c, "删除成功")
}

// AssignRolePermissions 分配角色权限
func (h *RoleHandler) AssignRolePermissions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色ID")
		return
	}

	// 系统角色不允许修改权限
	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "角色不存在")
		return
	}
	if role.IsSystem {
		response.BadRequest(c, "系统内置角色的权限不允许修改")
		return
	}

	var req AssignPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := h.roleRepo.AssignPermissions(c.Request.Context(), id, req.PermissionIDs); err != nil {
		response.InternalError(c, "分配权限失败")
		return
	}

	role, _ = h.roleRepo.GetByID(c.Request.Context(), id)
	response.Success(c, role)
}

// PermissionHandler 权限处理器
type PermissionHandler struct {
	permRepo *repository.PermissionRepository
}

// NewPermissionHandler 创建权限处理器
func NewPermissionHandler() *PermissionHandler {
	return &PermissionHandler{
		permRepo: repository.NewPermissionRepository(),
	}
}

// ListPermissions 获取权限列表
func (h *PermissionHandler) ListPermissions(c *gin.Context) {
	filter := repository.PermissionFilter{
		Search: c.Query("search"),
		Module: c.Query("module"),
		Name:   c.Query("name"),
		Code:   c.Query("code"),
	}

	perms, err := h.permRepo.ListWithFilter(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "获取权限列表失败")
		return
	}

	response.Success(c, perms)
}

// GetPermissionTree 获取权限树
func (h *PermissionHandler) GetPermissionTree(c *gin.Context) {
	perms, err := h.permRepo.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "获取权限列表失败")
		return
	}

	tree := make(map[string][]model.Permission)
	for _, p := range perms {
		tree[p.Module] = append(tree[p.Module], p)
	}

	response.Success(c, tree)
}

// 请求结构体
type UpdateUserRequest struct {
	DisplayName string `json:"display_name"`
	Phone       string `json:"phone"`
	Status      string `json:"status"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type AssignRolesRequest struct {
	RoleIDs []uuid.UUID `json:"role_ids" binding:"required"`
}

type UpdateRoleRequest struct {
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

type AssignPermissionsRequest struct {
	PermissionIDs []uuid.UUID `json:"permission_ids" binding:"required"`
}

// 辅助函数
func getQueryInt(c *gin.Context, key string, defaultValue int) int {
	if val := c.Query(key); val != "" {
		var result int
		if _, err := fmt.Sscanf(val, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}
