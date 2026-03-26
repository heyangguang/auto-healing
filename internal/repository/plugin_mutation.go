package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

type PluginConfigUpdate struct {
	Description         string
	Version             string
	Config              model.JSON
	FieldMapping        model.JSON
	SyncFilter          model.JSON
	SyncEnabled         bool
	SyncIntervalMinutes int
	NextSyncAt          *time.Time
	MaxFailures         int
}

func (r *PluginRepository) UpdateConfig(ctx context.Context, id uuid.UUID, update PluginConfigUpdate) error {
	return TenantDB(r.db, ctx).
		Model(&model.Plugin{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"description":           update.Description,
			"version":               update.Version,
			"config":                update.Config,
			"field_mapping":         update.FieldMapping,
			"sync_filter":           update.SyncFilter,
			"sync_enabled":          update.SyncEnabled,
			"sync_interval_minutes": update.SyncIntervalMinutes,
			"next_sync_at":          update.NextSyncAt,
			"max_failures":          update.MaxFailures,
			"updated_at":            time.Now(),
		}).Error
}
