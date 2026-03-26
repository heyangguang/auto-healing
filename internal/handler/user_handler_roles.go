package handler

import (
	"strings"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListSystemTenantRoles 平台级：获取系统级租户角色列表
func (h *RoleHandler) ListSystemTenantRoles(c *gin.Context) {
	roles, err := h.roleRepo.ListWithFilter(c.Request.Context(), repository.RoleFilter{Scope: "tenant"})
	if err != nil {
		response.InternalError(c, "获取角色列表失败")
		return
	}

	filtered := make([]model.Role, 0, len(roles))
	for _, role := range roles {
		if role.IsSystem && role.Name != "impersonation_accessor" {
			filtered = append(filtered, role)
		}
	}
	response.Success(c, filtered)
}

// ListRoles 平台级：获取所有角色列表（含统计信息）
func (h *RoleHandler) ListRoles(c *gin.Context) {
	roles, err := h.roleRepo.ListWithFilter(c.Request.Context(), repository.RoleFilter{
		Name:  c.Query("name"),
		Scope: "platform",
	})
	if err != nil {
		response.InternalError(c, "获取角色列表失败")
		return
	}
	response.Success(c, h.buildRoleStats(c, roles))
}

// ListTenantRoles 租户级：只返回租户可见角色，永远排除 platform_admin/super_admin
func (h *RoleHandler) ListTenantRoles(c *gin.Context) {
	tenantID, ok := requireTenantID(c, "ROLE")
	if !ok {
		return
	}
	roles, err := h.roleRepo.ListWithFilter(c.Request.Context(), repository.RoleFilter{
		Name:     c.Query("name"),
		Scope:    "tenant",
		TenantID: tenantID,
	})
	if err != nil {
		response.InternalError(c, "获取角色列表失败")
		return
	}
	response.Success(c, h.buildTenantRoleStats(c, roles, tenantID))
}

func (h *RoleHandler) buildRoleStats(c *gin.Context, roles []model.Role) []RoleWithStats {
	stats, _ := h.roleRepo.GetRoleStats(c.Request.Context())
	result := make([]RoleWithStats, len(roles))
	for i, role := range roles {
		result[i] = buildRoleWithStats(role, stats[role.ID.String()].UserCount)
	}
	return result
}

func (h *RoleHandler) buildTenantRoleStats(c *gin.Context, roles []model.Role, tenantID uuid.UUID) []RoleWithStats {
	stats, _ := h.roleRepo.GetTenantRoleStats(c.Request.Context(), tenantID)
	result := make([]RoleWithStats, len(roles))
	for i, role := range roles {
		result[i] = buildRoleWithStats(role, stats[role.ID.String()].UserCount)
	}
	return result
}

func buildRoleWithStats(role model.Role, userCount int64) RoleWithStats {
	return RoleWithStats{
		Role:            role,
		UserCount:       userCount,
		PermissionCount: int64(len(role.Permissions)),
	}
}

func isTenantRoleRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.FullPath(), "/api/v1/tenant/roles")
}

func (h *RoleHandler) getScopedRole(c *gin.Context, id uuid.UUID) (*model.Role, error) {
	if isTenantRoleRequest(c) {
		tenantID, ok := requireTenantID(c, "ROLE")
		if !ok {
			return nil, repository.ErrTenantContextRequired
		}
		return h.roleRepo.GetTenantRoleByID(c.Request.Context(), tenantID, id)
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		return nil, err
	}
	if role.Scope != "platform" {
		return nil, repository.ErrRoleNotFound
	}
	return role, nil
}

// CreateRole 创建角色
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if isTenantRoleRequest(c) {
		tenantID, ok := requireTenantID(c, "ROLE")
		if !ok {
			return
		}
		role.Scope = "tenant"
		role.IsSystem = false
		role.TenantID = &tenantID
	} else {
		role.Scope = "platform"
		role.IsSystem = false
		role.TenantID = nil
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

	role, err := h.getScopedRole(c, id)
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

	role, err := h.getScopedRole(c, id)
	if err != nil {
		response.NotFound(c, "角色不存在")
		return
	}
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

	role, err := h.getScopedRole(c, id)
	if err != nil {
		response.NotFound(c, "角色不存在")
		return
	}
	if role.IsSystem {
		response.BadRequest(c, "系统内置角色不允许删除")
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

	role, err := h.getScopedRole(c, id)
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

	role, err = h.getScopedRole(c, id)
	if err != nil {
		respondInternalError(c, "ROLE", "重新加载角色失败", err)
		return
	}
	response.Success(c, role)
}

// GetRoleUsers 获取角色下的关联用户
func (h *RoleHandler) GetRoleUsers(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	page, pageSize := normalizeRoleUsersPage(parsePagination(c, 20))
	users, total, err := h.roleRepo.GetRoleUsers(c.Request.Context(), id, page, pageSize, c.Query("name"))
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
	if _, err := h.getScopedRole(c, id); err != nil {
		response.NotFound(c, "角色不存在")
		return
	}

	tenantID, ok := requireTenantID(c, "ROLE")
	if !ok {
		return
	}
	page, pageSize := normalizeRoleUsersPage(parsePagination(c, 20))
	users, total, err := h.roleRepo.GetTenantRoleUsers(c.Request.Context(), id, tenantID, page, pageSize, c.Query("name"))
	if err != nil {
		response.InternalError(c, "获取角色用户失败")
		return
	}
	response.List(c, users, total, page, pageSize)
}

func normalizeRoleUsersPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
