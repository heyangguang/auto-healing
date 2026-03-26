package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	gitService "github.com/company/auto-healing/internal/service/git"
	"gorm.io/gorm"
)

const gitClaimLease = 40 * time.Minute

// GitScheduler Git 仓库同步调度器
type GitScheduler struct {
	gitSvc            *gitService.Service
	db                *gorm.DB
	interval          time.Duration
	lifecycle         *schedulerLifecycle
	running           bool
	mu                sync.Mutex
	loadReposNeedSync func(context.Context) ([]model.GitRepository, error)
	runRepoSync       func(context.Context, model.GitRepository)
	updateSyncState   func(context.Context, interface{}, map[string]interface{}) error
	claimRepoSync     func(context.Context, model.GitRepository) (bool, error)
}

// NewGitScheduler 创建 Git 同步调度器
func NewGitScheduler() *GitScheduler {
	s := &GitScheduler{
		gitSvc:   gitService.NewService(),
		db:       database.DB,
		interval: 60 * time.Second,
	}
	s.loadReposNeedSync = s.getReposNeedSync
	s.runRepoSync = s.syncRepo
	s.updateSyncState = s.updateGitSyncState
	s.claimRepoSync = s.claimRepo
	return s
}

// Start 启动调度器
func (s *GitScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.lifecycle = newSchedulerLifecycle()
	s.mu.Unlock()

	s.lifecycleGo(s.run)
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
	s.lifecycle = nil
	s.mu.Unlock()

	if lifecycle != nil {
		lifecycle.Stop()
	}
	logger.Sched("GIT").Info("Git 仓库同步调度器已停止")
}

// run 调度器主循环
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

// checkAndSync 检查并执行需要同步的仓库
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

	for _, repo := range repos {
		claimed, err := s.claimRepoSync(ctx, repo)
		if err != nil {
			logger.Sched("GIT").Error("认领 Git 仓库同步失败: %s (%s) - %v", repo.Name, repo.ID.String()[:8], err)
			continue
		}
		if !claimed {
			continue
		}
		if !s.dispatchRepoSync(repo) {
			s.rollbackRepoClaim(ctx, repo)
		}
	}
}

func (s *GitScheduler) dispatchRepoSync(repo model.GitRepository) bool {
	r := repo
	return s.lifecycleGo(func(rootCtx context.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				panicErr := fmt.Errorf("panic: %v", rec)
				shortID := r.ID.String()[:8]
				logger.Sched("GIT").Error("[%s] syncRepo panic: %v", shortID, rec)
				s.handleGitSyncError(withTenantContext(rootCtx, r.TenantID), r, shortID, time.Now().Add(resolveRepoSyncInterval(r)), panicErr)
			}
		}()
		s.runRepoSync(withTenantContext(rootCtx, r.TenantID), r)
	})
}

func (s *GitScheduler) lifecycleGo(fn func(context.Context)) bool {
	s.mu.Lock()
	lifecycle := s.lifecycle
	running := s.running
	s.mu.Unlock()
	if !running || lifecycle == nil {
		return false
	}
	return lifecycle.Go(fn)
}

func (s *GitScheduler) claimRepo(ctx context.Context, repo model.GitRepository) (bool, error) {
	if repo.NextSyncAt == nil {
		return false, nil
	}
	now := time.Now()
	nextSyncAt := now.Add(maxDuration(resolveRepoSyncInterval(repo), gitClaimLease))
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
	if err := s.updateSyncState(ctx, repo.ID, map[string]interface{}{
		"next_sync_at": repo.NextSyncAt,
	}); err != nil {
		logger.Sched("GIT").Warn("回滚 Git 仓库认领失败: %s (%s) - %v", repo.Name, repo.ID.String()[:8], err)
	}
}

func maxDuration(first, second time.Duration) time.Duration {
	if first >= second {
		return first
	}
	return second
}

// getReposNeedSync 获取需要同步的仓库列表
func (s *GitScheduler) getReposNeedSync(ctx context.Context) ([]model.GitRepository, error) {
	var repos []model.GitRepository
	now := time.Now()

	err := s.db.WithContext(ctx).
		Where("sync_enabled = ?", true).
		Where("next_sync_at IS NOT NULL").
		Where("next_sync_at <= ?", now).
		Find(&repos).Error

	return repos, err
}

// syncRepo 同步单个仓库
func (s *GitScheduler) syncRepo(ctx context.Context, repo model.GitRepository) {
	startTime := time.Now()
	shortID := repo.ID.String()[:8]
	logger.Sched("GIT").Info("[%s] 开始同步: %s", shortID, repo.Name)

	nextSyncAt := time.Now().Add(resolveRepoSyncInterval(repo))
	if err := s.gitSvc.SyncRepoWithTrigger(ctx, repo.ID, "scheduled"); err != nil {
		s.handleGitSyncError(ctx, repo, shortID, nextSyncAt, err)
		return
	}
	s.handleGitSyncSuccess(ctx, repo, shortID, nextSyncAt, time.Since(startTime))
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
