package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	"gorm.io/gorm"
)

type PluginAggregateStats struct {
	Total        int64
	ByType       map[string]int64
	ByStatus     map[string]int64
	SyncEnabled  int64
	SyncDisabled int64
}

func (r *PluginRepository) GetAggregateStats(ctx context.Context) (*PluginAggregateStats, error) {
	stats := &PluginAggregateStats{
		ByType:   map[string]int64{},
		ByStatus: map[string]int64{},
	}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	if err := newDB().Model(&model.Plugin{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	type typeCount struct {
		Type  string
		Count int64
	}
	var typeCounts []typeCount
	if err := newDB().Model(&model.Plugin{}).
		Select("type, count(*) as count").
		Group("type").
		Scan(&typeCounts).Error; err != nil {
		return nil, err
	}
	for _, item := range typeCounts {
		stats.ByType[item.Type] = item.Count
	}

	type statusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []statusCount
	if err := newDB().Model(&model.Plugin{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, err
	}
	for _, item := range statusCounts {
		stats.ByStatus[item.Status] = item.Count
	}

	if err := newDB().Model(&model.Plugin{}).Where("sync_enabled = ?", true).Count(&stats.SyncEnabled).Error; err != nil {
		return nil, err
	}
	stats.SyncDisabled = stats.Total - stats.SyncEnabled
	return stats, nil
}
