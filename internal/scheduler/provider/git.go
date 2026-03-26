package provider

import (
	"context"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	gitService "github.com/company/auto-healing/internal/service/git"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GitScheduler Git 仓库同步调度器
type GitScheduler struct {
	gitSvc              *gitService.Service
	db                  *gorm.DB
	interval            time.Duration
	lifecycle           *schedulerLifecycle
	inFlight            *inFlightSet
	now                 func() time.Time
	running             bool
	mu                  sync.Mutex
	loadReposNeedSync   func(context.Context) ([]model.GitRepository, error)
	runRepoSync         func(context.Context, model.GitRepository)
	syncRepoWithTrigger func(context.Context, uuid.UUID, string) error
	updateRepoState     func(context.Context, interface{}, map[string]interface{}) error
}

// NewGitScheduler 创建 Git 同步调度器
func NewGitScheduler() *GitScheduler {
	s := &GitScheduler{
		gitSvc:   gitService.NewService(),
		db:       database.DB,
		interval: 60 * time.Second,
		inFlight: newInFlightSet(),
		now:      time.Now,
	}
	s.loadReposNeedSync = s.getReposNeedSync
	s.runRepoSync = s.syncRepo
	s.syncRepoWithTrigger = s.gitSvc.SyncRepoWithTrigger
	s.updateRepoState = s.updateGitSyncState
	return s
}

// Start 启动调度器
func (s *GitScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	if s.lifecycle == nil || s.lifecycle.ctx.Err() != nil {
		s.lifecycle = newSchedulerLifecycle()
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

	lifecycle := s.lifecycleSnapshot()
	for _, repo := range repos {
		s.dispatchRepoSync(lifecycle, repo)
	}
}

func (s *GitScheduler) dispatchRepoSync(lifecycle *schedulerLifecycle, repo model.GitRepository) {
	if lifecycle == nil {
		return
	}
	if !s.inFlight.Start(repo.ID) {
		return
	}
	r := repo
	started := lifecycle.Go(func(rootCtx context.Context) {
		defer s.inFlight.Finish(r.ID)
		defer func() {
			if rec := recover(); rec != nil {
				logger.Sched("GIT").Error("[%s] syncRepo panic: %v", r.ID.String()[:8], rec)
			}
		}()
		s.runRepoSync(withTenantContext(rootCtx, r.TenantID), r)
	})
	if !started {
		s.inFlight.Finish(r.ID)
	}
}

func (s *GitScheduler) lifecycleSnapshot() *schedulerLifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lifecycle
}

// getReposNeedSync 获取需要同步的仓库列表
func (s *GitScheduler) getReposNeedSync(ctx context.Context) ([]model.GitRepository, error) {
	var repos []model.GitRepository
	now := s.now()

	err := s.db.WithContext(ctx).
		Where("sync_enabled = ?", true).
		Where("next_sync_at IS NOT NULL").
		Where("next_sync_at <= ?", now).
		Find(&repos).Error

	return filterDueRepos(repos, now), err
}

// syncRepo 同步单个仓库
func (s *GitScheduler) syncRepo(ctx context.Context, repo model.GitRepository) {
	startTime := s.now()
	shortID := repo.ID.String()[:8]
	logger.Sched("GIT").Info("[%s] 开始同步: %s", shortID, repo.Name)

	if err := s.syncRepoWithTrigger(ctx, repo.ID, "scheduled"); err != nil {
		completedAt := s.now()
		nextSyncAt := completedAt.Add(resolveRepoSyncInterval(repo))
		s.handleGitSyncError(ctx, repo, shortID, nextSyncAt, err)
		return
	}
	completedAt := s.now()
	nextSyncAt := completedAt.Add(resolveRepoSyncInterval(repo))
	s.handleGitSyncSuccess(ctx, repo, shortID, nextSyncAt, completedAt.Sub(startTime))
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func filterDueRepos(repos []model.GitRepository, now time.Time) []model.GitRepository {
	due := repos[:0]
	for _, repo := range repos {
		if repoSyncDue(repo, now) {
			due = append(due, repo)
		}
	}
	return due
}

func repoSyncDue(repo model.GitRepository, now time.Time) bool {
	return !lastSyncStillCoolingDown(repo.LastSyncAt, resolveRepoSyncInterval(repo), now)
}
