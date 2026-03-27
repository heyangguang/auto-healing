package repository

import (
	"context"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"time"

	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
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
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if err := countModel(newDB(), &projection.Plugin{}, &section.Total); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "active"), &projection.Plugin{}, &section.Active); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "inactive"), &projection.Plugin{}, &section.Inactive); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "error"), &projection.Plugin{}, &section.Error); err != nil {
		return nil, err
	}
	rate, err := calculatePluginSyncRate(newDB())
	if err != nil {
		return nil, err
	}
	section.SyncSuccessRate = rate
	if err := scanStatusCounts(newDB(), &projection.Plugin{}, "status", &section.ByStatus); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.Plugin{}, "type", &section.ByType); err != nil {
		return nil, err
	}
	if err := scanTrendPoints(newDB(), &projection.PluginSyncLog{}, "started_at", time.Now().AddDate(0, 0, -7), &section.SyncTrend7d); err != nil {
		return nil, err
	}
	recent, err := listPluginSyncs(newDB().Preload("Plugin", "tenant_id = ?", tenantID).Order("started_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentSyncs = recent
	errorPlugins, err := listPluginItems(newDB().Where("status = ?", "error"))
	if err != nil {
		return nil, err
	}
	section.ErrorPlugins = errorPlugins
	overview, err := listPluginItems(newDB().Order("name"))
	if err != nil {
		return nil, err
	}
	section.PluginOverview = overview
	return section, nil
}

func calculatePluginSyncRate(db *gorm.DB) (float64, error) {
	var total, success int64
	if err := countModel(db, &projection.PluginSyncLog{}, &total); err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	if err := countModel(db.Where("status = ?", "success"), &projection.PluginSyncLog{}, &success); err != nil {
		return 0, err
	}
	return float64(success) / float64(total) * 100, nil
}

func listPluginSyncs(query *gorm.DB) ([]SyncItem, error) {
	var logs []projection.PluginSyncLog
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
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
	return items, nil
}

func listPluginItems(query *gorm.DB) ([]PluginItem, error) {
	var plugins []projection.Plugin
	if err := query.Find(&plugins).Error; err != nil {
		return nil, err
	}
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
	return items, nil
}
