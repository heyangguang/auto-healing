package httpapi

import (
	"errors"

	"github.com/company/auto-healing/internal/middleware"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/company/auto-healing/internal/pkg/response"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateSystemWorkspace 创建系统工作区
func (h *DashboardHandler) CreateSystemWorkspace(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	var body struct {
		Name        string     `json:"name" binding:"required"`
		Description string     `json:"description"`
		Config      model.JSON `json:"config" binding:"required"`
	}
	if !parseDashboardBody(c, &body, "invalid request") {
		return
	}

	roleIDs, err := h.wsRepo.GetUserRoleIDs(c.Request.Context(), uid)
	if err != nil {
		respondInternalError(c, "DASHBOARD", "failed to load user roles", err)
		return
	}
	tenantRoleIDs, err := h.filterTenantRoleIDs(c, roleIDs)
	if err != nil {
		respondInternalError(c, "DASHBOARD", "failed to validate tenant roles", err)
		return
	}
	workspace := &model.SystemWorkspace{
		Name:        body.Name,
		Description: body.Description,
		Config:      body.Config,
		CreatedBy:   &uid,
	}
	if err := h.wsRepo.CreateAndAssignToRoles(c.Request.Context(), workspace, tenantRoleIDs); err != nil {
		respondInternalError(c, "DASHBOARD", "failed to create workspace", err)
		return
	}
	response.Success(c, workspace)
}

// ListSystemWorkspaces 获取所有系统工作区
func (h *DashboardHandler) ListSystemWorkspaces(c *gin.Context) {
	if !requireDashboardWorkspaceManage(c) {
		return
	}
	workspaces, err := h.wsRepo.List(c.Request.Context())
	if err != nil {
		respondInternalError(c, "DASHBOARD", "failed to list workspaces", err)
		return
	}
	response.Success(c, workspaces)
}

// UpdateSystemWorkspace 更新系统工作区
func (h *DashboardHandler) UpdateSystemWorkspace(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid workspace ID")
		return
	}
	existing, err := h.wsRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondInternalError(c, "DASHBOARD", "failed to get workspace", err)
		return
	}
	if existing == nil {
		response.NotFound(c, "workspace not found")
		return
	}

	var body struct {
		Name        *string    `json:"name"`
		Description *string    `json:"description"`
		Config      model.JSON `json:"config"`
	}
	if !parseDashboardBody(c, &body, "invalid request") {
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Description != nil {
		existing.Description = *body.Description
	}
	if body.Config != nil {
		existing.Config = body.Config
	}
	if err := h.wsRepo.Update(c.Request.Context(), existing); err != nil {
		respondInternalError(c, "DASHBOARD", "failed to update workspace", err)
		return
	}
	response.Success(c, existing)
}

// DeleteSystemWorkspace 删除系统工作区
func (h *DashboardHandler) DeleteSystemWorkspace(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid workspace ID")
		return
	}
	if err := h.wsRepo.Delete(c.Request.Context(), id); err != nil {
		respondInternalError(c, "DASHBOARD", "failed to delete workspace", err)
		return
	}
	response.Message(c, "workspace deleted")
}

// AssignRoleWorkspaces 为角色分配工作区
func (h *DashboardHandler) AssignRoleWorkspaces(c *gin.Context) {
	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		response.BadRequest(c, "invalid role ID")
		return
	}
	if _, err := h.requireTenantRole(c, roleID); err != nil {
		writeDashboardRoleScopeError(c, err)
		return
	}

	var body struct {
		WorkspaceIDs []string `json:"workspace_ids" binding:"required"`
	}
	if !parseDashboardBody(c, &body, "invalid request") {
		return
	}

	ids := make([]uuid.UUID, 0, len(body.WorkspaceIDs))
	for _, rawID := range body.WorkspaceIDs {
		workspaceID, err := uuid.Parse(rawID)
		if err != nil {
			response.BadRequest(c, "invalid workspace ID: "+rawID)
			return
		}
		ids = append(ids, workspaceID)
	}
	if err := h.wsRepo.AssignToRole(c.Request.Context(), roleID, ids); err != nil {
		respondInternalError(c, "DASHBOARD", "failed to assign workspaces", err)
		return
	}
	response.Message(c, "workspaces assigned")
}

// GetRoleWorkspaces 获取角色关联的工作区
func (h *DashboardHandler) GetRoleWorkspaces(c *gin.Context) {
	if !requireDashboardWorkspaceManage(c) {
		return
	}
	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		response.BadRequest(c, "invalid role ID")
		return
	}
	if _, err := h.requireTenantRole(c, roleID); err != nil {
		writeDashboardRoleScopeError(c, err)
		return
	}
	ids, err := h.wsRepo.GetRoleWorkspaceIDs(c.Request.Context(), roleID)
	if err != nil {
		respondInternalError(c, "DASHBOARD", "failed to get role workspaces", err)
		return
	}
	response.Success(c, map[string]interface{}{"workspace_ids": ids})
}

func requireDashboardWorkspaceManage(c *gin.Context) bool {
	if middleware.HasPermission(middleware.GetPermissions(c), "dashboard:workspace:manage") {
		return true
	}
	response.Forbidden(c, "dashboard workspace manage permission required")
	return false
}

func (h *DashboardHandler) filterTenantRoleIDs(c *gin.Context, roleIDs []uuid.UUID) ([]uuid.UUID, error) {
	tenantRoleIDs := make([]uuid.UUID, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		role, err := h.requireTenantRole(c, roleID)
		if errors.Is(err, accessrepo.ErrRoleNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if role != nil {
			tenantRoleIDs = append(tenantRoleIDs, role.ID)
		}
	}
	return tenantRoleIDs, nil
}

func (h *DashboardHandler) requireTenantRole(c *gin.Context, roleID uuid.UUID) (*model.Role, error) {
	tenantID, ok := requireTenantID(c, "DASHBOARD")
	if !ok {
		return nil, platformrepo.ErrTenantContextRequired
	}
	return h.roleRepo.GetTenantRoleByID(c.Request.Context(), tenantID, roleID)
}

func writeDashboardRoleScopeError(c *gin.Context, err error) {
	if errors.Is(err, accessrepo.ErrRoleNotFound) {
		response.NotFound(c, "role not found in current tenant")
		return
	}
	respondInternalError(c, "DASHBOARD", "failed to validate role scope", err)
}
