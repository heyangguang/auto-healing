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
)

// GitScheduler Git 仓库同步调度器
type GitScheduler struct {
	gitSvc   *gitService.Service
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
	mu       sync.Mutex
}

// NewGitScheduler 创建 Git 同步调度器
func NewGitScheduler() *GitScheduler {
	return &GitScheduler{
		gitSvc:   gitService.NewService(),
		interval: 60 * time.Second,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动调度器
func (s *GitScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()
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
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	logger.Sched("GIT").Info("Git 仓库同步调度器已停止")
}

// run 调度器主循环
func (s *GitScheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.checkAndSync()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndSync()
		}
	}
}

// checkAndSync 检查并执行需要同步的仓库
func (s *GitScheduler) checkAndSync() {
	ctx := context.Background()

	repos, err := s.getReposNeedSync(ctx)
	if err != nil {
		logger.Sched("GIT").Error("查询待同步 Git 仓库失败: %v", err)
		return
	}

	if len(repos) == 0 {
		return
	}

	logger.Sched("GIT").Info("发现 %d 个 Git 仓库需要同步", len(repos))

	for _, repo := range repos {
		go s.syncRepo(ctx, repo)
	}
}

// getReposNeedSync 获取需要同步的仓库列表
func (s *GitScheduler) getReposNeedSync(ctx context.Context) ([]model.GitRepository, error) {
	var repos []model.GitRepository
	now := time.Now()

	err := database.DB.WithContext(ctx).
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

	err := s.gitSvc.SyncRepo(ctx, repo.ID)
	duration := time.Since(startTime)

	if err != nil {
		logger.Sched("GIT").Error("[%s] 同步失败: %s - %v", shortID, repo.Name, err)
		return
	}

	interval, _ := time.ParseDuration(repo.SyncInterval)
	if interval == 0 {
		interval = time.Hour
	}
	nextSyncAt := time.Now().Add(interval)
	if err := s.updateNextSyncTime(ctx, repo.ID, nextSyncAt); err != nil {
		logger.Sched("GIT").Warn("[%s] 更新下次同步时间失败: %v", shortID, err)
	}

	logger.Sched("GIT").Info("[%s] 同步完成: %s | 分支: %s | 耗时: %v | 下次: %s",
		shortID,
		repo.Name,
		repo.DefaultBranch,
		duration,
		nextSyncAt.Format("15:04:05"),
	)
}

// updateNextSyncTime 更新下次同步时间
func (s *GitScheduler) updateNextSyncTime(ctx context.Context, repoID uuid.UUID, nextSyncAt time.Time) error {
	return database.DB.WithContext(ctx).
		Model(&model.GitRepository{}).
		Where("id = ?", repoID).
		Update("next_sync_at", nextSyncAt).Error
}
