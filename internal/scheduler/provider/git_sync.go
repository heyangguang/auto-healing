package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

func (s *GitScheduler) handleGitSyncError(ctx context.Context, repo model.GitRepository, shortID string, nextSyncAt time.Time, err error) {
	newCount := repo.ConsecutiveFailures + 1
	updates := map[string]interface{}{
		"consecutive_failures": newCount,
		"next_sync_at":         nextSyncAt,
	}

	if repo.MaxFailures > 0 && newCount >= repo.MaxFailures {
		updates["sync_enabled"] = false
		updates["next_sync_at"] = nil
		updates["pause_reason"] = fmt.Sprintf("连续失败 %d 次后自动暂停 (最后错误: %s)", newCount, truncateStr(err.Error(), 200))
	}

	if updateErr := s.persistRepoState(ctx, repo.ID, updates); updateErr != nil {
		logger.Sched("GIT").Error("[%s] 同步失败且状态落库失败: %s - %v | state_err=%v", shortID, repo.Name, err, updateErr)
		return
	}

	if repo.MaxFailures > 0 && newCount >= repo.MaxFailures {
		logger.Sched("GIT").Warn("[%s] ⚠ 连续失败 %d/%d 次，已自动暂停同步: %s", shortID, newCount, repo.MaxFailures, repo.Name)
	} else if repo.MaxFailures > 0 {
		logger.Sched("GIT").Warn("[%s] 同步失败 (%d/%d): %s - %v | 下次: %s",
			shortID, newCount, repo.MaxFailures, repo.Name, err, nextSyncAt.Format("15:04:05"))
	} else {
		logger.Sched("GIT").Warn("[%s] 同步失败 (第%d次): %s - %v | 下次: %s",
			shortID, newCount, repo.Name, err, nextSyncAt.Format("15:04:05"))
	}
}

func (s *GitScheduler) handleGitSyncSuccess(ctx context.Context, repo model.GitRepository, shortID string, nextSyncAt time.Time, duration time.Duration) {
	if err := s.persistRepoState(ctx, repo.ID, map[string]interface{}{
		"consecutive_failures": 0,
		"pause_reason":         "",
		"next_sync_at":         nextSyncAt,
	}); err != nil {
		logger.Sched("GIT").Error("[%s] 同步成功但状态落库失败: %s | state_err=%v", shortID, repo.Name, err)
		return
	}

	if repo.ConsecutiveFailures > 0 {
		logger.Sched("GIT").Info("[%s] 同步成功: %s | 失败计数已重置 (之前: %d) | 耗时: %v",
			shortID, repo.Name, repo.ConsecutiveFailures, duration)
		return
	}

	logger.Sched("GIT").Info("[%s] 同步完成: %s | 分支: %s | 耗时: %v | 下次: %s",
		shortID,
		repo.Name,
		repo.DefaultBranch,
		duration,
		nextSyncAt.Format("15:04:05"),
	)
}

func (s *GitScheduler) updateGitSyncState(ctx context.Context, repoID interface{}, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&model.GitRepository{}).Where("id = ?", repoID).Updates(updates).Error
}

func resolveRepoSyncInterval(repo model.GitRepository) time.Duration {
	interval, err := time.ParseDuration(repo.SyncInterval)
	if err == nil && interval > 0 {
		return interval
	}
	return time.Hour
}
