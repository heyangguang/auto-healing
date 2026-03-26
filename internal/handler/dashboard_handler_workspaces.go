package handler

import (
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
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
	workspace := &model.SystemWorkspace{
		Name:        body.Name,
		Description: body.Description,
		Config:      body.Config,
		CreatedBy:   &uid,
	}
	if err := h.wsRepo.CreateAndAssignToRoles(c.Request.Context(), workspace, roleIDs); err != nil {
		respondInternalError(c, "DASHBOARD", "failed to create workspace", err)
		return
	}
	response.Success(c, workspace)
}

// ListSystemWorkspaces 获取所有系统工作区
func (h *DashboardHandler) ListSystemWorkspaces(c *gin.Context) {
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
	response.Success(c, map[string]interface{}{"message": "workspace deleted"})
}

// AssignRoleWorkspaces 为角色分配工作区
func (h *DashboardHandler) AssignRoleWorkspaces(c *gin.Context) {
	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		response.BadRequest(c, "invalid role ID")
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
	response.Success(c, map[string]interface{}{"message": "workspaces assigned"})
}

// GetRoleWorkspaces 获取角色关联的工作区
func (h *DashboardHandler) GetRoleWorkspaces(c *gin.Context) {
	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		response.BadRequest(c, "invalid role ID")
		return
	}
	ids, err := h.wsRepo.GetRoleWorkspaceIDs(c.Request.Context(), roleID)
	if err != nil {
		respondInternalError(c, "DASHBOARD", "failed to get role workspaces", err)
		return
	}
	response.Success(c, map[string]interface{}{"workspace_ids": ids})
}
