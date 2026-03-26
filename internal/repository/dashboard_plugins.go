package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PluginSection struct {
	Total           int64         `json:"total"`
	Active          int64         `json:"active"`
	Inactive        int64         `json:"inactive"`
	Error           int64         `json:"error"`
	SyncSuccessRate float64       `json:"sync_success_rate"`
	ByStatus        []StatusCount `json:"by_status"`
	ByType          []StatusCount `json:"by_type"`
	SyncTrend7d     []TrendPoint  `json:"sync_trend_7d"`
	RecentSyncs     []SyncItem    `json:"recent_syncs"`
	ErrorPlugins    []PluginItem  `json:"error_plugins"`
	PluginOverview  []PluginItem  `json:"plugin_overview"`
}

type SyncItem struct {
	ID         uuid.UUID `json:"id"`
	PluginName string    `json:"plugin_name"`
	Status     string    `json:"status"`
	SyncType   string    `json:"sync_type"`
	StartedAt  time.Time `json:"started_at"`
}

type PluginItem struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	LastSyncAt *time.Time `json:"last_sync_at"`
}

func (r *DashboardRepository) GetPluginSection(ctx context.Context) (*PluginSection, error) {
	section := &PluginSection{
		ByStatus: []StatusCount{},
		ByType:   []StatusCount{},
	}
	db := r.tenantDB(ctx)

	countModel(db, &model.Plugin{}, &section.Total)
	countModel(db.Where("status = ?", "active"), &model.Plugin{}, &section.Active)
	countModel(db.Where("status = ?", "inactive"), &model.Plugin{}, &section.Inactive)
	countModel(db.Where("status = ?", "error"), &model.Plugin{}, &section.Error)
	section.SyncSuccessRate = calculatePluginSyncRate(db)
	scanStatusCounts(db, &model.Plugin{}, "status", &section.ByStatus)
	scanStatusCounts(db, &model.Plugin{}, "type", &section.ByType)
	scanTrendPoints(db, &model.PluginSyncLog{}, "started_at", time.Now().AddDate(0, 0, -7), &section.SyncTrend7d)
	section.RecentSyncs = listPluginSyncs(db.Preload("Plugin").Order("started_at DESC").Limit(10))
	section.ErrorPlugins = listPluginItems(db.Where("status = ?", "error"))
	section.PluginOverview = listPluginItems(db.Order("name"))
	return section, nil
}

func calculatePluginSyncRate(db *gorm.DB) float64 {
	var total, success int64
	countModel(db, &model.PluginSyncLog{}, &total)
	if total == 0 {
		return 0
	}
	countModel(db.Where("status = ?", "success"), &model.PluginSyncLog{}, &success)
	return float64(success) / float64(total) * 100
}

func listPluginSyncs(query *gorm.DB) []SyncItem {
	var logs []model.PluginSyncLog
	query.Find(&logs)
	items := make([]SyncItem, 0, len(logs))
	for _, log := range logs {
		name := ""
		if log.Plugin.Name != "" {
			name = log.Plugin.Name
		}
		items = append(items, SyncItem{
			ID:         log.ID,
			PluginName: name,
			Status:     log.Status,
			SyncType:   log.SyncType,
			StartedAt:  log.StartedAt,
		})
	}
	return items
}

func listPluginItems(query *gorm.DB) []PluginItem {
	var plugins []model.Plugin
	query.Find(&plugins)
	items := make([]PluginItem, 0, len(plugins))
	for _, plugin := range plugins {
		items = append(items, PluginItem{
			ID:         plugin.ID,
			Name:       plugin.Name,
			Type:       plugin.Type,
			Status:     plugin.Status,
			LastSyncAt: plugin.LastSyncAt,
		})
	}
	return items
}
