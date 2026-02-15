package handler

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/service/plugin"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PluginHandler 插件处理器
type PluginHandler struct {
	pluginSvc   *plugin.Service
	incidentSvc *plugin.IncidentService
}

// NewPluginHandler 创建插件处理器
func NewPluginHandler() *PluginHandler {
	return &PluginHandler{
		pluginSvc:   plugin.NewService(),
		incidentSvc: plugin.NewIncidentService(),
	}
}

// ListPlugins 获取插件列表
func (h *PluginHandler) ListPlugins(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	pluginType := c.Query("type")
	status := c.Query("status")
	search := c.Query("search")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")

	plugins, total, err := h.pluginSvc.ListPlugins(c.Request.Context(), page, pageSize, pluginType, status, search, sortBy, sortOrder)
	if err != nil {
		response.InternalError(c, "获取插件列表失败")
		return
	}

	response.List(c, plugins, total, page, pageSize)
}

// GetPluginStats 获取插件统计数据
func (h *PluginHandler) GetPluginStats(c *gin.Context) {
	stats, err := h.pluginSvc.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "获取插件统计失败")
		return
	}

	response.Success(c, stats)
}

// CreatePlugin 创建插件
func (h *PluginHandler) CreatePlugin(c *gin.Context) {
	var req CreatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	p, err := h.pluginSvc.CreatePlugin(c.Request.Context(), req.ToModel())
	if err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	response.Created(c, p)
}

// GetPlugin 获取插件详情
func (h *PluginHandler) GetPlugin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	p, err := h.pluginSvc.GetPlugin(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "插件不存在")
		return
	}

	response.Success(c, p)
}

// UpdatePlugin 更新插件
func (h *PluginHandler) UpdatePlugin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	var req UpdatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	p, err := h.pluginSvc.UpdatePlugin(c.Request.Context(), id, req.Description, req.Version, req.Config, req.FieldMapping, req.SyncFilter, req.SyncEnabled, req.SyncIntervalMinutes)
	if err != nil {
		response.InternalError(c, "更新失败")
		return
	}

	response.Success(c, p)
}

// DeletePlugin 删除插件
func (h *PluginHandler) DeletePlugin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	if err := h.pluginSvc.DeletePlugin(c.Request.Context(), id); err != nil {
		response.InternalError(c, "删除失败")
		return
	}

	response.Message(c, "删除成功")
}

// TestPlugin 测试插件连接（只测试，不改变状态）
func (h *PluginHandler) TestPlugin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	if err := h.pluginSvc.TestConnection(c.Request.Context(), id); err != nil {
		response.BadRequest(c, "连接测试失败: "+err.Error())
		return
	}

	response.Message(c, "连接测试成功")
}

// ActivatePlugin 激活插件（测试成功后激活）
func (h *PluginHandler) ActivatePlugin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	if err := h.pluginSvc.Activate(c.Request.Context(), id); err != nil {
		response.BadRequest(c, "激活失败: "+err.Error())
		return
	}

	response.Message(c, "插件已激活")
}

// DeactivatePlugin 停用插件
func (h *PluginHandler) DeactivatePlugin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	if err := h.pluginSvc.Deactivate(c.Request.Context(), id); err != nil {
		response.InternalError(c, "停用失败: "+err.Error())
		return
	}

	response.Message(c, "插件已停用")
}

// SyncPlugin 触发插件同步
func (h *PluginHandler) SyncPlugin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	syncLog, err := h.pluginSvc.TriggerSync(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "触发同步失败")
		return
	}

	response.Success(c, syncLog)
}

// GetPluginSyncLogs 获取插件同步日志
func (h *PluginHandler) GetPluginSyncLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)

	logs, total, err := h.pluginSvc.GetSyncLogs(c.Request.Context(), id, page, pageSize)
	if err != nil {
		response.InternalError(c, "获取同步日志失败")
		return
	}

	response.List(c, logs, total, page, pageSize)
}

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
	sourcePluginName := c.Query("source_plugin_name")
	search := c.Query("search")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")

	var pluginID *uuid.UUID
	if pidStr := c.Query("plugin_id"); pidStr != "" {
		pid, err := uuid.Parse(pidStr)
		if err == nil {
			pluginID = &pid
		}
	}

	var hasPlugin *bool
	if hpStr := c.Query("has_plugin"); hpStr != "" {
		hp := hpStr == "true"
		hasPlugin = &hp
	}

	incidents, total, err := h.incidentSvc.ListIncidents(c.Request.Context(), page, pageSize, pluginID, status, healingStatus, severity, sourcePluginName, search, hasPlugin, sortBy, sortOrder)
	if err != nil {
		response.InternalError(c, "获取工单列表失败")
		return
	}

	response.List(c, incidents, total, page, pageSize)
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
		response.NotFound(c, "工单不存在")
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
		response.InternalError(c, "关闭工单失败: "+err.Error())
		return
	}

	response.Success(c, resp)
}

// ResetIncidentScan 重置工单扫描状态
// 将工单的 scanned 设为 false，清除匹配的规则和流程实例关联
// 这样工单会被自愈调度器重新扫描
func (h *PluginHandler) ResetIncidentScan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的工单ID")
		return
	}

	if err := h.incidentSvc.ResetScan(c.Request.Context(), id); err != nil {
		response.InternalError(c, "重置扫描状态失败: "+err.Error())
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
		response.InternalError(c, "批量重置失败: "+err.Error())
		return
	}

	response.Success(c, resp)
}
