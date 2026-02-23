package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	pluginService "github.com/company/auto-healing/internal/service/plugin"
)

// Scheduler 定时调度器
type Scheduler struct {
	pluginSvc *pluginService.Service
	cmdbSvc   *pluginService.CMDBService
	interval  time.Duration
	stopCh    chan struct{}
	wg        sync.WaitGroup
	running   bool
	mu        sync.Mutex
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	return &Scheduler{
		pluginSvc: pluginService.NewService(),
		cmdbSvc:   pluginService.NewCMDBService(),
		interval:  30 * time.Second,
		stopCh:    make(chan struct{}),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()
	logger.Sched("SYNC").Info("插件同步调度器已启动 (检查间隔: %v)", s.interval)
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	logger.Sched("SYNC").Info("插件同步调度器已停止")
}

// run 调度器主循环
func (s *Scheduler) run() {
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

// checkAndSync 检查并执行需要同步的插件
func (s *Scheduler) checkAndSync() {
	ctx := context.Background()

	// 检查并恢复到期的维护
	if count, err := s.cmdbSvc.CheckExpiredMaintenance(ctx); err != nil {
		logger.Sched("SYNC").Warn("检查维护到期失败: %v", err)
	} else if count > 0 {
		logger.Sched("SYNC").Info("自动恢复 %d 个维护到期的配置项", count)
	}

	plugins, err := s.getPluginsNeedSync(ctx)
	if err != nil {
		logger.Sched("SYNC").Error("查询待同步插件失败: %v", err)
		return
	}

	if len(plugins) == 0 {
		return
	}

	logger.Sched("SYNC").Info("发现 %d 个插件需要同步", len(plugins))

	for _, plugin := range plugins {
		go func(p model.Plugin) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Sched("SYNC").Error("[%s] syncPlugin panic: %v", p.ID.String()[:8], rec)
				}
			}()
			s.syncPlugin(ctx, p)
		}(plugin)
	}
}

// getPluginsNeedSync 获取需要同步的插件列表
func (s *Scheduler) getPluginsNeedSync(ctx context.Context) ([]model.Plugin, error) {
	var plugins []model.Plugin
	now := time.Now()

	err := database.DB.WithContext(ctx).
		Where("sync_enabled = ?", true).
		Where("status = ?", "active").
		Where("next_sync_at IS NOT NULL").
		Where("next_sync_at <= ?", now).
		Find(&plugins).Error

	return plugins, err
}

// syncPlugin 同步单个插件
func (s *Scheduler) syncPlugin(ctx context.Context, plugin model.Plugin) {
	startTime := time.Now()
	shortID := plugin.ID.String()[:8]
	logger.Sched("SYNC").Info("[%s] 开始同步: %s", shortID, plugin.Name)

	syncLog, err := s.pluginSvc.TriggerSyncSync(ctx, plugin.ID)

	// 计算下次同步时间（保持原始间隔）
	nextSyncAt := time.Now().Add(time.Duration(plugin.SyncIntervalMinutes) * time.Minute)

	if err != nil {
		// 连续失败计数 +1
		newCount := plugin.ConsecutiveFailures + 1
		updates := map[string]interface{}{
			"consecutive_failures": newCount,
			"next_sync_at":         nextSyncAt,
		}

		// 检查是否需要自动暂停（max_failures > 0 才启用）
		if plugin.MaxFailures > 0 && newCount >= plugin.MaxFailures {
			updates["sync_enabled"] = false
			updates["next_sync_at"] = nil
			updates["pause_reason"] = fmt.Sprintf("连续失败 %d 次后自动暂停 (最后错误: %s)", newCount, truncateStr(err.Error(), 200))
			logger.Sched("SYNC").Warn("[%s] ⚠ 连续失败 %d/%d 次，已自动暂停同步: %s",
				shortID, newCount, plugin.MaxFailures, plugin.Name)
		} else {
			if plugin.MaxFailures > 0 {
				logger.Sched("SYNC").Warn("[%s] 同步失败 (%d/%d): %s - %v",
					shortID, newCount, plugin.MaxFailures, plugin.Name, err)
			} else {
				logger.Sched("SYNC").Warn("[%s] 同步失败 (第%d次): %s - %v",
					shortID, newCount, plugin.Name, err)
			}
		}

		database.DB.WithContext(ctx).Model(&model.Plugin{}).Where("id = ?", plugin.ID).Updates(updates)
		return
	}

	// 更新下次同步时间
	updates := map[string]interface{}{
		"next_sync_at": nextSyncAt,
	}

	// 检查 syncLog 结果：非 success 也视为失败
	duration := time.Since(startTime)
	if syncLog.Status != "success" {
		newCount := plugin.ConsecutiveFailures + 1
		updates["consecutive_failures"] = newCount

		if plugin.MaxFailures > 0 && newCount >= plugin.MaxFailures {
			updates["sync_enabled"] = false
			updates["next_sync_at"] = nil
			updates["pause_reason"] = fmt.Sprintf("连续失败 %d 次后自动暂停 (状态: %s)", newCount, syncLog.Status)
			logger.Sched("SYNC").Warn("[%s] ⚠ 连续失败 %d/%d 次，已自动暂停同步: %s",
				shortID, newCount, plugin.MaxFailures, plugin.Name)
		} else {
			cleanError := strings.ReplaceAll(syncLog.ErrorMessage, "\n", " ")
			cleanError = strings.ReplaceAll(cleanError, "\r", "")
			if len(cleanError) > 200 {
				cleanError = cleanError[:200] + "..."
			}
			logger.Sched("SYNC").Warn("[%s] 同步异常 (%d/%d): %s | 状态: %s | 获取: %d条 | 处理: %d条 | 失败: %d条 | 错误: %s | 耗时: %v",
				shortID,
				newCount, func() int {
					if plugin.MaxFailures > 0 {
						return plugin.MaxFailures
					}
					return 0
				}(),
				plugin.Name,
				syncLog.Status,
				syncLog.RecordsFetched,
				syncLog.RecordsProcessed,
				syncLog.RecordsFailed,
				cleanError,
				duration,
			)
		}

		database.DB.WithContext(ctx).Model(&model.Plugin{}).Where("id = ?", plugin.ID).Updates(updates)
		return
	}

	// 成功 → 重置失败计数
	updates["consecutive_failures"] = 0
	updates["pause_reason"] = ""

	database.DB.WithContext(ctx).Model(&model.Plugin{}).Where("id = ?", plugin.ID).Updates(updates)

	if plugin.ConsecutiveFailures > 0 {
		logger.Sched("SYNC").Info("[%s] 同步成功: %s | 失败计数已重置 (之前: %d) | 耗时: %v",
			shortID, plugin.Name, plugin.ConsecutiveFailures, duration)
	} else {
		newCount := 0
		updatedCount := 0
		if syncLog.Details != nil {
			if v, ok := syncLog.Details["new_count"].(int); ok {
				newCount = v
			}
			if v, ok := syncLog.Details["updated_count"].(int); ok {
				updatedCount = v
			}
		}
		logger.Sched("SYNC").Info("[%s] 同步完成: %s | 获取: %d条 | 新增: %d条 | 更新: %d条 | 失败: %d条 | 耗时: %v | 下次: %s",
			shortID,
			plugin.Name,
			syncLog.RecordsFetched,
			newCount,
			updatedCount,
			syncLog.RecordsFailed,
			duration,
			nextSyncAt.Format("15:04:05"),
		)
	}
}
