package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	gitService "github.com/company/auto-healing/internal/service/git"
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
		go func(r model.GitRepository) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Sched("GIT").Error("[%s] syncRepo panic: %v", r.ID.String()[:8], rec)
				}
			}()
			// 注入仓库所属租户的 context，确保 GitSyncLog 和 Playbook 操作写入正确租户
			repoCtx := context.Background()
			if r.TenantID != nil {
				repoCtx = repository.WithTenantID(repoCtx, *r.TenantID)
			}
			s.syncRepo(repoCtx, r)
		}(repo)
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

	err := s.gitSvc.SyncRepoWithTrigger(ctx, repo.ID, "scheduled")
	duration := time.Since(startTime)

	if err != nil {
		// 连续失败计数 +1
		newCount := repo.ConsecutiveFailures + 1
		updates := map[string]interface{}{
			"consecutive_failures": newCount,
		}

		// 计算下次同步时间（保持原始间隔）
		interval, _ := time.ParseDuration(repo.SyncInterval)
		if interval == 0 {
			interval = time.Hour
		}
		nextSyncAt := time.Now().Add(interval)
		updates["next_sync_at"] = nextSyncAt

		// 检查是否需要自动暂停（max_failures > 0 才启用）
		if repo.MaxFailures > 0 && newCount >= repo.MaxFailures {
			updates["sync_enabled"] = false
			updates["next_sync_at"] = nil
			updates["pause_reason"] = fmt.Sprintf("连续失败 %d 次后自动暂停 (最后错误: %s)", newCount, truncateStr(err.Error(), 200))
			logger.Sched("GIT").Warn("[%s] ⚠ 连续失败 %d/%d 次，已自动暂停同步: %s",
				shortID, newCount, repo.MaxFailures, repo.Name)
		} else {
			if repo.MaxFailures > 0 {
				logger.Sched("GIT").Warn("[%s] 同步失败 (%d/%d): %s - %v | 下次: %s",
					shortID, newCount, repo.MaxFailures, repo.Name, err, nextSyncAt.Format("15:04:05"))
			} else {
				logger.Sched("GIT").Warn("[%s] 同步失败 (第%d次): %s - %v | 下次: %s",
					shortID, newCount, repo.Name, err, nextSyncAt.Format("15:04:05"))
			}
		}

		database.DB.WithContext(ctx).Model(&model.GitRepository{}).Where("id = ?", repo.ID).Updates(updates)
		return
	}

	// 成功 → 重置失败计数
	interval, _ := time.ParseDuration(repo.SyncInterval)
	if interval == 0 {
		interval = time.Hour
	}
	nextSyncAt := time.Now().Add(interval)

	updates := map[string]interface{}{
		"consecutive_failures": 0,
		"pause_reason":         "",
		"next_sync_at":         nextSyncAt,
	}
	database.DB.WithContext(ctx).Model(&model.GitRepository{}).Where("id = ?", repo.ID).Updates(updates)

	if repo.ConsecutiveFailures > 0 {
		logger.Sched("GIT").Info("[%s] 同步成功: %s | 失败计数已重置 (之前: %d) | 耗时: %v",
			shortID, repo.Name, repo.ConsecutiveFailures, duration)
	} else {
		logger.Sched("GIT").Info("[%s] 同步完成: %s | 分支: %s | 耗时: %v | 下次: %s",
			shortID,
			repo.Name,
			repo.DefaultBranch,
			duration,
			nextSyncAt.Format("15:04:05"),
		)
	}
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
