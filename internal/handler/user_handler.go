package handler

import (
	"fmt"
	"strconv"

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

// ListUsers 获取平台管理员用户列表
// 只返回拥有 platform_admin 或 super_admin 角色的用户
func (h *UserHandler) ListUsers(c *gin.Context) {
	params := &repository.UserListParams{
		Page:         getQueryInt(c, "page", 1),
		PageSize:     getQueryInt(c, "page_size", 20),
		Status:       c.Query("status"),
		Username:     GetStringFilter(c, "username"),
		Email:        GetStringFilter(c, "email"),
		DisplayName:  GetStringFilter(c, "display_name"),
		CreatedFrom:  c.Query("created_from"),
		CreatedTo:    c.Query("created_to"),
		SortField:    c.Query("sort_field"),
		SortOrder:    c.Query("sort_order"),
		PlatformOnly: true, // 平台接口只展示平台级角色用户
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
	name := c.Query("name")
	status := c.DefaultQuery("status", "active") // 默认只返回活跃用户

	users, err := h.userRepo.ListSimple(c.Request.Context(), name, status)
	if err != nil {
		response.InternalError(c, "获取用户列表失败")
		return
	}

	response.Success(c, users)
}

// CreateUser 创建平台用户，支持选择平台角色（不传则默认 platform_admin）
func (h *UserHandler) CreateUser(c *gin.Context) {
	// 扩展请求结构，支持 role_id
	var body struct {
		authService.RegisterRequest
		RoleID *uuid.UUID `json:"role_id"` // 可选：指定平台角色 ID
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), &body.RegisterRequest)
	if err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	// 确定要分配的角色
	var assignRoleID uuid.UUID
	if body.RoleID != nil {
		// 前端指定了角色，校验是否为 platform scope 角色
		role, err := h.roleRepo.GetByID(c.Request.Context(), *body.RoleID)
		if err != nil {
			response.BadRequest(c, "指定的角色不存在")
			return
		}
		if role.Scope != "platform" {
			response.BadRequest(c, "只能分配平台级别角色")
			return
		}
		assignRoleID = role.ID
	} else {
		// 默认分配 platform_admin
		platformAdminRole, err := h.roleRepo.GetByName(c.Request.Context(), "platform_admin")
		if err != nil {
			response.InternalError(c, "获取默认角色失败")
			return
		}
		assignRoleID = platformAdminRole.ID
	}
	_ = h.userRepo.AssignRoles(c.Request.Context(), user.ID, []uuid.UUID{assignRoleID})

	// 返回完整用户信息（含角色）
	userWithRoles, _ := h.userRepo.GetByID(c.Request.Context(), user.ID)
	if userWithRoles != nil {
		response.Created(c, userWithRoles)
	} else {
		response.Created(c, user)
	}
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

	// 保护最后一个平台管理员：不允许禁用
	if req.Status != "" && req.Status != "active" {
		if h.isLastPlatformAdmin(c, id) {
			response.BadRequest(c, "系统中必须保留至少一个可用的平台管理员，无法禁用")
			return
		}
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

	// 如果传了 role_id，变更用户角色
	if req.RoleID != nil {
		role, err := h.roleRepo.GetByID(c.Request.Context(), *req.RoleID)
		if err != nil {
			response.BadRequest(c, "指定的角色不存在")
			return
		}
		if role.Scope != "platform" {
			response.BadRequest(c, "只能分配平台级别角色")
			return
		}
		// 保护最后一个平台管理员：不允许降级角色
		if role.Name != "platform_admin" && h.isLastPlatformAdmin(c, id) {
			response.BadRequest(c, "系统中必须保留至少一个平台管理员，无法变更角色")
			return
		}
		if err := h.userRepo.AssignRoles(c.Request.Context(), user.ID, []uuid.UUID{role.ID}); err != nil {
			response.InternalError(c, "角色变更失败")
			return
		}
	}

	// 返回含角色的完整信息
	userWithRoles, _ := h.userRepo.GetByID(c.Request.Context(), user.ID)
	if userWithRoles != nil {
		response.Success(c, userWithRoles)
	} else {
		response.Success(c, user)
	}
}

// DeleteUser 删除平台管理员账号
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

	// 保护：最后一个 platform_admin 不允许删除
	platformAdmins, _, err := h.userRepo.List(c.Request.Context(), &repository.UserListParams{
		Page:         1,
		PageSize:     2,
		PlatformOnly: true,
	})
	if err == nil && len(platformAdmins) <= 1 {
		// 检查被删除的用户是否是 platform_admin
		for _, u := range platformAdmins {
			if u.ID == id {
				response.BadRequest(c, "系统中必须保留至少一个平台管理员，无法删除")
				return
			}
		}
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

	// 保护最后一个平台管理员：检查新角色列表是否仍包含 platform_admin
	if h.isLastPlatformAdmin(c, id) {
		hasPlatformAdmin := false
		for _, roleID := range req.RoleIDs {
			if role, err := h.roleRepo.GetByID(c.Request.Context(), roleID); err == nil && role.Name == "platform_admin" {
				hasPlatformAdmin = true
				break
			}
		}
		if !hasPlatformAdmin {
			response.BadRequest(c, "系统中必须保留至少一个平台管理员，无法移除 platform_admin 角色")
			return
		}
	}

	if err := h.userRepo.AssignRoles(c.Request.Context(), id, req.RoleIDs); err != nil {
		response.InternalError(c, "分配角色失败")
		return
	}

	userWithRoles, _ := h.userRepo.GetByID(c.Request.Context(), id)
	response.Success(c, userWithRoles)
}

// isLastPlatformAdmin 判断指定用户是否是最后一个平台管理员
func (h *UserHandler) isLastPlatformAdmin(c *gin.Context, userID uuid.UUID) bool {
	platformAdmins, _, err := h.userRepo.List(c.Request.Context(), &repository.UserListParams{
		Page:         1,
		PageSize:     2,
		PlatformOnly: true,
		Status:       "active",
	})
	if err != nil || len(platformAdmins) > 1 {
		return false // 查询失败或有多个管理员，不阻止
	}
	// 只剩一个且就是目标用户
	return len(platformAdmins) == 1 && platformAdmins[0].ID == userID
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

// ListSystemTenantRoles 平台级：获取系统级租户角色列表
// 供平台管理页面（如租户成员管理）在无租户上下文时使用
func (h *RoleHandler) ListSystemTenantRoles(c *gin.Context) {
	filter := repository.RoleFilter{
		Scope: "tenant",
	}

	roles, err := h.roleRepo.ListWithFilter(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "获取角色列表失败")
		return
	}

	// 过滤掉 impersonation_accessor 和非系统角色（只保留可供用户选择的系统角色）
	filtered := make([]model.Role, 0, len(roles))
	for _, r := range roles {
		if r.IsSystem && r.Name != "impersonation_accessor" {
			filtered = append(filtered, r)
		}
	}

	response.Success(c, filtered)
}

// ListRoles 平台级：获取所有角色列表（含统计信息）
func (h *RoleHandler) ListRoles(c *gin.Context) {
	filter := repository.RoleFilter{
		Name:  c.Query("name"),
		Scope: "platform",
	}

	roles, err := h.roleRepo.ListWithFilter(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "获取角色列表失败")
		return
	}

	response.Success(c, h.buildRoleStats(c, roles))
}

// ListTenantRoles 租户级：只返回租户可见角色，永远排除 platform_admin/super_admin
func (h *RoleHandler) ListTenantRoles(c *gin.Context) {
	tenantID := repository.TenantIDFromContext(c.Request.Context())
	filter := repository.RoleFilter{
		Name:     c.Query("name"),
		Scope:    "tenant",
		TenantID: tenantID,
	}

	roles, err := h.roleRepo.ListWithFilter(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "获取角色列表失败")
		return
	}

	response.Success(c, h.buildTenantRoleStats(c, roles, tenantID))
}

// RoleWithStats 角色+统计
type RoleWithStats struct {
	model.Role
	UserCount       int64 `json:"user_count"`
	PermissionCount int64 `json:"permission_count"`
}

// buildRoleStats 构建角色列表（含统计信息）— 平台级
func (h *RoleHandler) buildRoleStats(c *gin.Context, roles []model.Role) []RoleWithStats {
	stats, _ := h.roleRepo.GetRoleStats(c.Request.Context())

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
	return result
}

// buildTenantRoleStats 构建角色列表（含统计信息）— 租户级
func (h *RoleHandler) buildTenantRoleStats(c *gin.Context, roles []model.Role, tenantID uuid.UUID) []RoleWithStats {
	stats, _ := h.roleRepo.GetTenantRoleStats(c.Request.Context(), tenantID)

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
	return result
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

// GetRoleUsers 获取角色下的关联用户
func (h *RoleHandler) GetRoleUsers(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	name := c.Query("name")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := h.roleRepo.GetRoleUsers(c.Request.Context(), id, page, pageSize, name)
	if err != nil {
		response.InternalError(c, "获取角色用户失败")
		return
	}

	response.List(c, users, total, page, pageSize)
}

// GetTenantRoleUsers 获取租户下角色关联的用户
func (h *RoleHandler) GetTenantRoleUsers(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	tenantID := repository.TenantIDFromContext(c.Request.Context())
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	name := c.Query("name")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := h.roleRepo.GetTenantRoleUsers(c.Request.Context(), id, tenantID, page, pageSize, name)
	if err != nil {
		response.InternalError(c, "获取角色用户失败")
		return
	}

	response.List(c, users, total, page, pageSize)
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
	DisplayName string     `json:"display_name"`
	Phone       string     `json:"phone"`
	Status      string     `json:"status"`
	RoleID      *uuid.UUID `json:"role_id"` // 可选：变更平台角色
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
