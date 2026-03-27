package httpapi

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetOverview 获取 Dashboard 概览数据
func (h *DashboardHandler) GetOverview(c *gin.Context) {
	sectionsParam := c.DefaultQuery("sections", "")
	if sectionsParam == "" {
		response.BadRequest(c, "sections parameter is required")
		return
	}
	sections := strings.Split(sectionsParam, ",")
	unauthorized := dashboardUnauthorizedSections(sections, middleware.GetPermissions(c))
	if len(unauthorized) > 0 {
		response.Forbidden(c, fmt.Sprintf("no permission to access dashboard sections: %s", strings.Join(unauthorized, ", ")))
		return
	}

	result, err := h.loadDashboardSections(c.Request.Context(), sections, middleware.GetPermissions(c))
	if err != nil {
		respondInternalError(c, "DASHBOARD", "failed to get dashboard overview", err)
		return
	}
	response.Success(c, result)
}

func (h *DashboardHandler) loadDashboardSections(ctx context.Context, sections []string, permissions []string) (map[string]interface{}, error) {
	loaders := make(map[string]dashboardSectionFunc)
	for _, rawSection := range sections {
		section := strings.TrimSpace(rawSection)
		loader := dashboardSectionLoader(h, section, permissions)
		if section == "" || loader == nil {
			continue
		}
		loaders[section] = loader
	}
	return loadDashboardSectionsFromLoaders(ctx, loaders)
}

func loadDashboardSectionsFromLoaders(ctx context.Context, loaders map[string]dashboardSectionFunc) (map[string]interface{}, error) {
	state := newConcurrentSectionState(len(loaders))
	var wg sync.WaitGroup

	for section, loader := range loaders {
		wg.Add(1)
		go func(section string, loader dashboardSectionFunc) {
			defer wg.Done()
			data, err := safeSectionLoad(func() (interface{}, error) {
				return loader(ctx)
			})
			if err != nil {
				state.addError(section, err)
				return
			}
			state.addResult(section, data)
		}(section, loader)
	}

	wg.Wait()
	return state.resultAndError()
}

// GetConfig 获取用户 Dashboard 配置（合并角色分配的系统工作区）
func (h *DashboardHandler) GetConfig(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	config, err := h.repo.GetConfigByUserID(c.Request.Context(), uid)
	if err != nil {
		respondInternalError(c, "DASHBOARD", "failed to get config", err)
		return
	}
	roleWorkspaces, err := h.wsRepo.GetWorkspacesByUserRoles(c.Request.Context(), uid)
	if err != nil {
		respondInternalError(c, "DASHBOARD", "failed to get role workspaces", err)
		return
	}

	result := map[string]interface{}{"config": map[string]interface{}{}}
	if config != nil {
		result["config"] = config.Config
	}
	result["system_workspaces"] = buildSystemWorkspaceList(roleWorkspaces)
	response.Success(c, result)
}

func buildSystemWorkspaceList(workspaces []model.SystemWorkspace) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(workspaces))
	for _, workspace := range workspaces {
		items = append(items, map[string]interface{}{
			"id":          workspace.ID,
			"name":        workspace.Name,
			"description": workspace.Description,
			"config":      workspace.Config,
			"is_system":   true,
			"is_readonly": true,
			"is_default":  workspace.IsDefault,
		})
	}
	return items
}

// SaveConfig 保存用户 Dashboard 配置
func (h *DashboardHandler) SaveConfig(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	var body map[string]interface{}
	if !parseDashboardBody(c, &body, "invalid request body") {
		return
	}
	configJSON, err := toDashboardJSON(body)
	if err != nil {
		response.BadRequest(c, "failed to parse config: "+err.Error())
		return
	}
	if err := h.repo.UpsertConfig(c.Request.Context(), uid, configJSON); err != nil {
		respondInternalError(c, "DASHBOARD", "failed to save config", err)
		return
	}
	response.Success(c, map[string]interface{}{"message": "config saved successfully"})
}
