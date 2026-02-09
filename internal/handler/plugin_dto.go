package handler

import (
	"github.com/company/auto-healing/internal/model"
)

// ==================== Plugin DTO ====================

// CreatePluginRequest 创建插件请求
type CreatePluginRequest struct {
	Name                string     `json:"name" binding:"required"`
	Type                string     `json:"type" binding:"required"`
	Description         string     `json:"description"`
	Version             string     `json:"version"`
	Config              model.JSON `json:"config" binding:"required"`
	FieldMapping        model.JSON `json:"field_mapping"`
	SyncFilter          model.JSON `json:"sync_filter"`
	SyncEnabled         bool       `json:"sync_enabled"`
	SyncIntervalMinutes int        `json:"sync_interval_minutes"`
}

// ToModel 转换为 Model
func (r *CreatePluginRequest) ToModel() *model.Plugin {
	version := r.Version
	if version == "" {
		version = "1.0.0"
	}
	fieldMapping := r.FieldMapping
	if fieldMapping == nil {
		fieldMapping = model.JSON{}
	}

	return &model.Plugin{
		Name:                r.Name,
		Type:                r.Type,
		Description:         r.Description,
		Version:             version,
		Config:              r.Config,
		FieldMapping:        fieldMapping,
		SyncFilter:          r.SyncFilter,
		SyncEnabled:         r.SyncEnabled,
		SyncIntervalMinutes: r.SyncIntervalMinutes,
		Status:              "inactive",
	}
}

// UpdatePluginRequest 更新插件请求
type UpdatePluginRequest struct {
	Description         string     `json:"description"`
	Version             string     `json:"version"`
	Config              model.JSON `json:"config"`
	FieldMapping        model.JSON `json:"field_mapping"`
	SyncFilter          model.JSON `json:"sync_filter"`
	SyncEnabled         *bool      `json:"sync_enabled"`
	SyncIntervalMinutes *int       `json:"sync_interval_minutes"`
}

// ApplyTo 应用更新到模型
func (r *UpdatePluginRequest) ApplyTo(plugin *model.Plugin) {
	if r.Description != "" {
		plugin.Description = r.Description
	}
	if r.Version != "" {
		plugin.Version = r.Version
	}
	if r.Config != nil {
		plugin.Config = r.Config
	}
	if r.FieldMapping != nil {
		plugin.FieldMapping = r.FieldMapping
	}
	if r.SyncEnabled != nil {
		plugin.SyncEnabled = *r.SyncEnabled
	}
	if r.SyncFilter != nil {
		plugin.SyncFilter = r.SyncFilter
	}
	if r.SyncIntervalMinutes != nil {
		plugin.SyncIntervalMinutes = *r.SyncIntervalMinutes
	}
}

// CloseIncidentRequest 关闭工单请求
type CloseIncidentRequest struct {
	Resolution  string `json:"resolution"`
	WorkNotes   string `json:"work_notes"`
	CloseCode   string `json:"close_code"`
	CloseStatus string `json:"close_status"`
}

// GetCloseStatus 获取关闭状态（默认 resolved）
func (r *CloseIncidentRequest) GetCloseStatus() string {
	if r.CloseStatus == "" {
		return "resolved"
	}
	return r.CloseStatus
}
