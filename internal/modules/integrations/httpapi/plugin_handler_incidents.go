package httpapi

import (
	"errors"

	"github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetIncidentStats 获取工单统计数据
func (h *PluginHandler) GetIncidentStats(c *gin.Context) {
	incidentRepo := repository.NewIncidentRepository()
	stats, err := incidentRepo.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "获取工单统计失败")
		return
	}
	response.Success(c, stats)
}

// ListIncidents 获取工单列表
func (h *PluginHandler) ListIncidents(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	status := c.Query("status")
	healingStatus := c.Query("healing_status")
	severity := c.Query("severity")
	sourcePluginName := GetStringFilter(c, "source_plugin_name")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")
	pluginID, hasPlugin := parseIncidentPluginFilters(c)
	externalID := GetStringFilter(c, "external_id")
	scopes := BuildSchemaScopes(c, incidentSearchSchema, "source_plugin_name", "external_id")

	incidents, total, err := h.incidentSvc.ListIncidents(c.Request.Context(), page, pageSize, pluginID, status, healingStatus, severity, sourcePluginName, query.StringFilter{}, hasPlugin, sortBy, sortOrder, externalID, scopes...)
	if err != nil {
		response.InternalError(c, "获取工单列表失败")
		return
	}
	response.List(c, incidents, total, page, pageSize)
}

func parseIncidentPluginFilters(c *gin.Context) (*uuid.UUID, *bool) {
	var pluginID *uuid.UUID
	if pidStr := c.Query("plugin_id"); pidStr != "" {
		if pid, err := uuid.Parse(pidStr); err == nil {
			pluginID = &pid
		}
	}

	var hasPlugin *bool
	if hpStr := c.Query("has_plugin"); hpStr != "" {
		hp := hpStr == "true"
		hasPlugin = &hp
	}
	return pluginID, hasPlugin
}

// GetIncident 获取工单详情
func (h *PluginHandler) GetIncident(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的工单ID")
		return
	}

	incident, err := h.incidentSvc.GetIncident(c.Request.Context(), id)
	if err != nil {
		respondPluginIncidentError(c, "获取工单详情失败", err)
		return
	}
	response.Success(c, incident)
}

// CloseIncident 关闭工单
func (h *PluginHandler) CloseIncident(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的工单ID")
		return
	}

	var req CloseIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.incidentSvc.CloseIncident(c.Request.Context(), id, req.Resolution, req.WorkNotes, req.CloseCode, req.GetCloseStatus())
	if err != nil {
		respondPluginIncidentError(c, "关闭工单失败", err)
		return
	}
	response.Success(c, resp)
}

// ResetIncidentScan 重置工单扫描状态
func (h *PluginHandler) ResetIncidentScan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的工单ID")
		return
	}

	if err := h.incidentSvc.ResetScan(c.Request.Context(), id); err != nil {
		respondPluginIncidentError(c, "重置扫描状态失败", err)
		return
	}
	response.Message(c, "工单扫描状态已重置，将被重新扫描")
}

// BatchResetIncidentScan 批量重置工单扫描状态
func (h *PluginHandler) BatchResetIncidentScan(c *gin.Context) {
	var req plugin.BatchResetScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.incidentSvc.BatchResetScan(c.Request.Context(), req.IDs, req.HealingStatus)
	if err != nil {
		if errors.Is(err, plugin.ErrBatchResetScanScopeRequired) {
			response.BadRequest(c, err.Error())
			return
		}
		respondInternalError(c, "PLUGIN", "批量重置失败", err)
		return
	}
	response.Success(c, resp)
}

func respondPluginIncidentError(c *gin.Context, publicMsg string, err error) {
	if errors.Is(err, repository.ErrIncidentNotFound) {
		response.NotFound(c, "工单不存在")
		return
	}
	respondInternalError(c, "PLUGIN", publicMsg, err)
}
