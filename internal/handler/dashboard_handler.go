package handler

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DashboardHandler Dashboard 处理器
type DashboardHandler struct {
	repo   *repository.DashboardRepository
	wsRepo *repository.WorkspaceRepository
}

// NewDashboardHandler 创建 Dashboard 处理器
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{
		repo:   repository.NewDashboardRepository(),
		wsRepo: repository.NewWorkspaceRepository(),
	}
}

// GetOverview 获取 Dashboard 概览数据
// GET /api/v1/dashboard/overview?sections=incidents,cmdb,healing,...
func (h *DashboardHandler) GetOverview(c *gin.Context) {
	sectionsParam := c.DefaultQuery("sections", "")
	if sectionsParam == "" {
		response.BadRequest(c, "sections parameter is required")
		return
	}

	sections := strings.Split(sectionsParam, ",")
	ctx := c.Request.Context()

	// 用 map 收集每个 section 的结果
	result := make(map[string]interface{})
	var mu sync.Mutex
	var wg sync.WaitGroup
	var lastErr error

	for _, sec := range sections {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}

		wg.Add(1)
		go func(section string) {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					mu.Lock()
					lastErr = fmt.Errorf("section %s panic: %v", section, rec)
					mu.Unlock()
				}
			}()

			var data interface{}
			var err error

			switch section {
			case "incidents":
				data, err = h.repo.GetIncidentSection(ctx)
			case "cmdb":
				data, err = h.repo.GetCMDBSection(ctx)
			case "healing":
				data, err = h.repo.GetHealingSection(ctx)
			case "execution":
				data, err = h.repo.GetExecutionSection(ctx)
			case "plugins":
				data, err = h.repo.GetPluginSection(ctx)
			case "notifications":
				data, err = h.repo.GetNotificationSection(ctx)
			case "git":
				data, err = h.repo.GetGitSection(ctx)
			case "playbooks":
				data, err = h.repo.GetPlaybookSection(ctx)
			case "secrets":
				data, err = h.repo.GetSecretsSection(ctx)
			case "users":
				data, err = h.repo.GetUsersSection(ctx)
			default:
				// 忽略未知 section
				return
			}

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				lastErr = err
				return
			}
			result[section] = data
		}(sec)
	}

	wg.Wait()

	if lastErr != nil {
		response.InternalError(c, "failed to get dashboard overview: "+lastErr.Error())
		return
	}

	response.Success(c, result)
}

// GetConfig 获取用户 Dashboard 配置（合并角色分配的系统工作区）
// GET /api/v1/dashboard/config
func (h *DashboardHandler) GetConfig(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		response.BadRequest(c, "invalid user ID")
		return
	}

	config, err := h.repo.GetConfigByUserID(c.Request.Context(), uid)
	if err != nil {
		response.InternalError(c, "failed to get config: "+err.Error())
		return
	}

	// 获取用户角色关联的系统工作区
	roleWorkspaces, err := h.wsRepo.GetWorkspacesByUserRoles(c.Request.Context(), uid)
	if err != nil {
		response.InternalError(c, "failed to get role workspaces: "+err.Error())
		return
	}

	// 构建响应
	result := map[string]interface{}{}
	if config != nil {
		result["config"] = config.Config
	} else {
		result["config"] = map[string]interface{}{}
	}

	// 添加系统工作区列表
	sysWsList := make([]map[string]interface{}, 0, len(roleWorkspaces))
	for _, ws := range roleWorkspaces {
		sysWsList = append(sysWsList, map[string]interface{}{
			"id":          ws.ID,
			"name":        ws.Name,
			"description": ws.Description,
			"config":      ws.Config,
			"is_system":   true,
			"is_readonly": true,
			"is_default":  ws.IsDefault,
		})
	}
	result["system_workspaces"] = sysWsList

	response.Success(c, result)
}

// SaveConfig 保存用户 Dashboard 配置
// PUT /api/v1/dashboard/config
func (h *DashboardHandler) SaveConfig(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		response.BadRequest(c, "invalid user ID")
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	// 将 body 转为 model.JSON
	// 先序列化再反序列化以确保类型正确
	configBytes, err := json.Marshal(body)
	if err != nil {
		response.BadRequest(c, "failed to serialize config: "+err.Error())
		return
	}

	var configJSON model.JSON
	if err := json.Unmarshal(configBytes, &configJSON); err != nil {
		response.BadRequest(c, "failed to parse config: "+err.Error())
		return
	}

	if err := h.repo.UpsertConfig(c.Request.Context(), uid, configJSON); err != nil {
		response.InternalError(c, "failed to save config: "+err.Error())
		return
	}

	response.Success(c, map[string]interface{}{
		"message": "config saved successfully",
	})
}

// ==================== 系统工作区管理 ====================

// CreateSystemWorkspace 创建系统工作区（自动分配给当前用户的角色）
// POST /api/v1/dashboard/workspaces
func (h *DashboardHandler) CreateSystemWorkspace(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		response.BadRequest(c, "invalid user ID")
		return
	}

	var body struct {
		Name        string     `json:"name" binding:"required"`
		Description string     `json:"description"`
		Config      model.JSON `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	ws := &model.SystemWorkspace{
		Name:        body.Name,
		Description: body.Description,
		Config:      body.Config,
		CreatedBy:   &uid,
	}

	// 自动分配给当前用户的所有角色
	roleIDs, err := h.wsRepo.GetUserRoleIDs(c.Request.Context(), uid)
	if err != nil {
		response.InternalError(c, "failed to load user roles: "+err.Error())
		return
	}

	if err := h.wsRepo.CreateAndAssignToRoles(c.Request.Context(), ws, roleIDs); err != nil {
		response.InternalError(c, "failed to create workspace: "+err.Error())
		return
	}

	response.Success(c, ws)
}

// ListSystemWorkspaces 获取所有系统工作区
// GET /api/v1/dashboard/workspaces
func (h *DashboardHandler) ListSystemWorkspaces(c *gin.Context) {
	workspaces, err := h.wsRepo.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to list workspaces: "+err.Error())
		return
	}
	response.Success(c, workspaces)
}

// UpdateSystemWorkspace 更新系统工作区
// PUT /api/v1/dashboard/workspaces/:id
func (h *DashboardHandler) UpdateSystemWorkspace(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid workspace ID")
		return
	}

	existing, err := h.wsRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to get workspace: "+err.Error())
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
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
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
		response.InternalError(c, "failed to update workspace: "+err.Error())
		return
	}

	response.Success(c, existing)
}

// DeleteSystemWorkspace 删除系统工作区
// DELETE /api/v1/dashboard/workspaces/:id
func (h *DashboardHandler) DeleteSystemWorkspace(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid workspace ID")
		return
	}

	if err := h.wsRepo.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "failed to delete workspace: "+err.Error())
		return
	}

	response.Success(c, map[string]interface{}{"message": "workspace deleted"})
}

// AssignRoleWorkspaces 为角色分配工作区
// PUT /api/v1/dashboard/roles/:roleId/workspaces
func (h *DashboardHandler) AssignRoleWorkspaces(c *gin.Context) {
	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		response.BadRequest(c, "invalid role ID")
		return
	}

	var body struct {
		WorkspaceIDs []string `json:"workspace_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	ids := make([]uuid.UUID, 0, len(body.WorkspaceIDs))
	for _, idStr := range body.WorkspaceIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			response.BadRequest(c, "invalid workspace ID: "+idStr)
			return
		}
		ids = append(ids, id)
	}

	if err := h.wsRepo.AssignToRole(c.Request.Context(), roleID, ids); err != nil {
		response.InternalError(c, "failed to assign workspaces: "+err.Error())
		return
	}

	response.Success(c, map[string]interface{}{"message": "workspaces assigned"})
}

// GetRoleWorkspaces 获取角色关联的工作区
// GET /api/v1/dashboard/roles/:roleId/workspaces
func (h *DashboardHandler) GetRoleWorkspaces(c *gin.Context) {
	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		response.BadRequest(c, "invalid role ID")
		return
	}

	ids, err := h.wsRepo.GetRoleWorkspaceIDs(c.Request.Context(), roleID)
	if err != nil {
		response.InternalError(c, "failed to get role workspaces: "+err.Error())
		return
	}

	response.Success(c, map[string]interface{}{"workspace_ids": ids})
}
