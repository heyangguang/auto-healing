package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	gitService "github.com/company/auto-healing/internal/modules/integrations/service/git"
	"github.com/company/auto-healing/internal/pkg/logger"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const gitClaimLease = 40 * time.Minute

// GitScheduler Git 仓库同步调度器
type GitScheduler struct {
	gitSvc              *gitService.Service
	db                  *gorm.DB
	interval            time.Duration
	lifecycle           *schedulerx.Lifecycle
	inFlight            *schedulerx.InFlightSet
	now                 func() time.Time
	running             bool
	mu                  sync.Mutex
	loadReposNeedSync   func(context.Context) ([]model.GitRepository, error)
	runRepoSync         func(context.Context, model.GitRepository)
	syncRepoWithTrigger func(context.Context, uuid.UUID, string) error
	updateSyncState     func(context.Context, interface{}, map[string]interface{}) error
	updateRepoState     func(context.Context, interface{}, map[string]interface{}) error
	claimRepoSync       func(context.Context, model.GitRepository) (bool, error)
}

type GitSchedulerDeps struct {
	GitService *gitService.Service
	DB         *gorm.DB
	Interval   time.Duration
	Lifecycle  *schedulerx.Lifecycle
	InFlight   *schedulerx.InFlightSet
	Now        func() time.Time
}

func DefaultGitSchedulerDepsWithDB(db *gorm.DB) GitSchedulerDeps {
	return GitSchedulerDeps{
		GitService: gitService.NewServiceWithDB(db),
		DB:         db,
		Interval:   60 * time.Second,
		InFlight:   schedulerx.NewInFlightSet(),
		Now:        time.Now,
	}
}

func NewGitSchedulerWithDeps(deps GitSchedulerDeps) *GitScheduler {
	switch {
	case deps.GitService == nil:
		panic("integrations git scheduler requires git service")
	}
	if deps.Interval == 0 {
		deps.Interval = 60 * time.Second
	}
	if deps.InFlight == nil {
		deps.InFlight = schedulerx.NewInFlightSet()
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	s := &GitScheduler{
		gitSvc:    deps.GitService,
		db:        deps.DB,
		interval:  deps.Interval,
		lifecycle: deps.Lifecycle,
		inFlight:  deps.InFlight,
		now:       deps.Now,
	}
	s.loadReposNeedSync = s.getReposNeedSync
	s.runRepoSync = s.syncRepo
	s.syncRepoWithTrigger = s.gitSvc.SyncRepoWithTrigger
	s.updateRepoState = s.updateGitSyncState
	s.claimRepoSync = s.claimRepo
	return s
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
	return !schedulerx.LastSyncStillCoolingDown(repo.LastSyncAt, resolveRepoSyncInterval(repo), now)
}
