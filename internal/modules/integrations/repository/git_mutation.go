package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/google/uuid"
)

type GitRepositoryConfigUpdate struct {
	DefaultBranch string
	AuthType      string
	AuthConfig    model.JSON
	SyncEnabled   bool
	SyncInterval  string
	NextSyncAt    *time.Time
	MaxFailures   int
}

func (r *GitRepositoryRepository) UpdateConfig(ctx context.Context, id uuid.UUID, update GitRepositoryConfigUpdate) error {
	updates := map[string]any{
		"default_branch": update.DefaultBranch,
		"auth_type":      update.AuthType,
		"auth_config":    update.AuthConfig,
		"sync_enabled":   update.SyncEnabled,
		"sync_interval":  update.SyncInterval,
		"next_sync_at":   update.NextSyncAt,
		"max_failures":   update.MaxFailures,
		"updated_at":     time.Now(),
	}
	if update.SyncEnabled {
		updates["consecutive_failures"] = 0
		updates["pause_reason"] = ""
		updates["error_message"] = ""
	}
	return TenantDB(r.db, ctx).
		Model(&model.GitRepository{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *GitRepositoryRepository) UpdateLocalPath(ctx context.Context, id uuid.UUID, localPath string) error {
	return TenantDB(r.db, ctx).
		Model(&model.GitRepository{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"local_path": localPath,
			"updated_at": time.Now(),
		}).Error
}

func (r *GitRepositoryRepository) UpdateSyncState(ctx context.Context, id uuid.UUID, status, errorMessage, lastCommitID string, lastSyncAt, nextSyncAt *time.Time) error {
	updates := map[string]any{
		"status":         status,
		"error_message":  errorMessage,
		"last_commit_id": lastCommitID,
		"last_sync_at":   lastSyncAt,
		"next_sync_at":   nextSyncAt,
		"updated_at":     time.Now(),
	}
	if status == "ready" && errorMessage == "" {
		updates["consecutive_failures"] = 0
		updates["pause_reason"] = ""
	}
	return TenantDB(r.db, ctx).
		Model(&model.GitRepository{}).
		Where("id = ?", id).
		Updates(updates).Error
}
