package handler

import (
	"strconv"

	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListCMDBItems 获取 CMDB 列表
func (h *CMDBHandler) ListCMDBItems(c *gin.Context) {
	page, pageSize := parseCMDBPagination(c)
	pluginID, hasPlugin := parseCMDBPluginFilters(c)
	scopes := BuildSchemaScopes(c, cmdbSearchSchema)

	items, total, err := h.cmdbSvc.ListCMDBItems(
		c.Request.Context(),
		page,
		pageSize,
		pluginID,
		c.Query("type"),
		c.Query("status"),
		c.Query("environment"),
		"",
		query.StringFilter{},
		hasPlugin,
		c.Query("sort_by"),
		c.Query("sort_order"),
		scopes...,
	)
	if err != nil {
		respondInternalError(c, "CMDB", "获取 CMDB 列表失败", err)
		return
	}
	response.List(c, items, total, page, pageSize)
}

func parseCMDBPagination(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	if raw := c.Query("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	return page, pageSize
}

func parseCMDBPluginFilters(c *gin.Context) (*uuid.UUID, *bool) {
	var pluginID *uuid.UUID
	if raw := c.Query("plugin_id"); raw != "" {
		if parsed, err := uuid.Parse(raw); err == nil {
			pluginID = &parsed
		}
	}

	var hasPlugin *bool
	if raw := c.Query("has_plugin"); raw != "" {
		value := raw == "true"
		hasPlugin = &value
	}
	return pluginID, hasPlugin
}

// ListCMDBItemIDs 获取符合筛选条件的配置项 ID 列表
func (h *CMDBHandler) ListCMDBItemIDs(c *gin.Context) {
	pluginID, hasPlugin := parseCMDBPluginFilters(c)
	items, total, err := h.cmdbSvc.ListCMDBItemIDs(
		c.Request.Context(),
		pluginID,
		c.Query("type"),
		c.Query("status"),
		c.Query("environment"),
		c.Query("source_plugin_name"),
		hasPlugin,
	)
	if err != nil {
		respondInternalError(c, "CMDB", "获取 CMDB ID 列表失败", err)
		return
	}
	response.Success(c, map[string]interface{}{"items": items, "total": total})
}

// GetCMDBItem 获取 CMDB 详情
func (h *CMDBHandler) GetCMDBItem(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID格式")
		return
	}

	item, err := h.cmdbSvc.GetCMDBItem(c.Request.Context(), id)
	if err != nil {
		respondCMDBItemError(c, "获取 CMDB 详情失败", err)
		return
	}
	response.Success(c, item)
}

// GetCMDBStats 获取 CMDB 统计信息
func (h *CMDBHandler) GetCMDBStats(c *gin.Context) {
	stats, err := h.cmdbSvc.GetCMDBStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "CMDB", "获取统计信息失败", err)
		return
	}
	response.Success(c, stats)
}
