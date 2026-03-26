package provider

import (
	"context"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	gitService "github.com/company/auto-healing/internal/service/git"
	"gorm.io/gorm"
)

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

	s.lifecycle.Go(s.run)
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

	lifecycle.Stop()
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
		s.dispatchRepoSync(repo)
	}
}

func (s *GitScheduler) dispatchRepoSync(repo model.GitRepository) {
	r := repo
	s.lifecycle.Go(func(rootCtx context.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Sched("GIT").Error("[%s] syncRepo panic: %v", r.ID.String()[:8], rec)
			}
		}()
		s.runRepoSync(withTenantContext(rootCtx, r.TenantID), r)
	})
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
