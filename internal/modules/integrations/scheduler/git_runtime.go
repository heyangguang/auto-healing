package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
)

// Start 启动调度器
func (s *GitScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	if s.lifecycle == nil || s.lifecycle.Context().Err() != nil {
		s.lifecycle = schedulerx.NewLifecycle()
	}
	lifecycle := s.lifecycle
	s.running = true
	s.mu.Unlock()

	lifecycle.Go(s.run)
	logger.Sched("GIT").Info("Git 仓库同步调度器已启动 (检查间隔: %v)", s.interval)
}

// Stop 停止调度器
func (s *GitScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	lifecycle := s.lifecycle
	s.mu.Unlock()

	if lifecycle != nil {
		lifecycle.Stop()
	}
	logger.Sched("GIT").Info("Git 仓库同步调度器已停止")
}

func (s *GitScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.checkAndSync(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAndSync(ctx)
		}
	}
}

func (s *GitScheduler) checkAndSync(ctx context.Context) {
	repos, err := s.loadReposNeedSync(ctx)
	if err != nil {
		logger.Sched("GIT").Error("查询待同步 Git 仓库失败: %v", err)
		return
	}
	if len(repos) == 0 {
		return
	}

	logger.Sched("GIT").Info("发现 %d 个 Git 仓库需要同步", len(repos))

	lifecycle := s.lifecycleSnapshot()
	for _, repo := range repos {
		claimed, err := s.claimRepoSync(ctx, repo)
		if err != nil {
			logger.Sched("GIT").Error("认领 Git 仓库同步失败: %s (%s) - %v", repo.Name, repo.ID.String()[:8], err)
			continue
		}
		if !claimed {
			continue
		}
		if !s.dispatchRepoSync(lifecycle, repo) {
			s.rollbackRepoClaim(ctx, repo)
		}
	}
}

func (s *GitScheduler) dispatchRepoSync(lifecycle *schedulerx.Lifecycle, repo model.GitRepository) bool {
	if lifecycle == nil {
		return false
	}
	if !s.inFlight.Start(repo.ID) {
		return false
	}

	r := repo
	started := lifecycle.Go(func(rootCtx context.Context) {
		defer s.inFlight.Finish(r.ID)
		defer func() {
			if rec := recover(); rec != nil {
				panicErr := fmt.Errorf("panic: %v", rec)
				shortID := r.ID.String()[:8]
				logger.Sched("GIT").Error("[%s] syncRepo panic: %v", shortID, rec)
				s.handleGitSyncError(schedulerx.WithTenantContext(rootCtx, r.TenantID), r, shortID, s.now().Add(resolveRepoSyncInterval(r)), panicErr)
			}
		}()
		s.runRepoSync(schedulerx.WithTenantContext(rootCtx, r.TenantID), r)
	})
	if !started {
		s.inFlight.Finish(r.ID)
	}
	return started
}

func (s *GitScheduler) lifecycleSnapshot() *schedulerx.Lifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lifecycle
}

func (s *GitScheduler) claimRepo(ctx context.Context, repo model.GitRepository) (bool, error) {
	if repo.NextSyncAt == nil {
		return false, nil
	}
	now := s.now()
	nextSyncAt := now.Add(schedulerx.MaxDuration(resolveRepoSyncInterval(repo), gitClaimLease))
	result := s.db.WithContext(ctx).
		Model(&model.GitRepository{}).
		Where("id = ? AND sync_enabled = ? AND next_sync_at IS NOT NULL AND next_sync_at <= ?", repo.ID, true, now).
		Update("next_sync_at", nextSyncAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (s *GitScheduler) rollbackRepoClaim(ctx context.Context, repo model.GitRepository) {
	if repo.NextSyncAt == nil {
		return
	}
	if err := s.persistRepoState(ctx, repo.ID, map[string]interface{}{
		"next_sync_at": repo.NextSyncAt,
	}); err != nil {
		logger.Sched("GIT").Warn("回滚 Git 仓库认领失败: %s (%s) - %v", repo.Name, repo.ID.String()[:8], err)
	}
}

func (s *GitScheduler) persistRepoState(ctx context.Context, repoID interface{}, updates map[string]interface{}) error {
	if s.updateSyncState != nil {
		return s.updateSyncState(ctx, repoID, updates)
	}
	if s.updateRepoState != nil {
		return s.updateRepoState(ctx, repoID, updates)
	}
	return nil
}
