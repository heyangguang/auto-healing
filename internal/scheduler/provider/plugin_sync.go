package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

// syncPlugin 同步单个插件
func (s *Scheduler) syncPlugin(ctx context.Context, plugin model.Plugin) {
	startTime := s.now()
	shortID := plugin.ID.String()[:8]
	logger.Sched("SYNC").Info("[%s] 开始同步: %s", shortID, plugin.Name)

	syncLog, err := s.triggerPluginSync(ctx, plugin.ID)
	if err != nil {
		completedAt := s.now()
		nextSyncAt := completedAt.Add(time.Duration(plugin.SyncIntervalMinutes) * time.Minute)
		s.handlePluginSyncError(ctx, plugin, shortID, nextSyncAt, err)
		return
	}
	completedAt := s.now()
	nextSyncAt := completedAt.Add(time.Duration(plugin.SyncIntervalMinutes) * time.Minute)
	s.handlePluginSyncResult(ctx, plugin, syncLog, shortID, nextSyncAt, completedAt.Sub(startTime))
}

func (s *Scheduler) handlePluginSyncError(ctx context.Context, plugin model.Plugin, shortID string, nextSyncAt time.Time, err error) {
	newCount := plugin.ConsecutiveFailures + 1
	updates := map[string]interface{}{
		"consecutive_failures": newCount,
		"next_sync_at":         nextSyncAt,
	}

	if plugin.MaxFailures > 0 && newCount >= plugin.MaxFailures {
		updates["sync_enabled"] = false
		updates["next_sync_at"] = nil
		updates["pause_reason"] = fmt.Sprintf("连续失败 %d 次后自动暂停 (最后错误: %s)", newCount, truncateStr(err.Error(), 200))
		logger.Sched("SYNC").Warn("[%s] ⚠ 连续失败 %d/%d 次，已自动暂停同步: %s",
			shortID, newCount, plugin.MaxFailures, plugin.Name)
	} else if plugin.MaxFailures > 0 {
		logger.Sched("SYNC").Warn("[%s] 同步失败 (%d/%d): %s - %v",
			shortID, newCount, plugin.MaxFailures, plugin.Name, err)
	} else {
		logger.Sched("SYNC").Warn("[%s] 同步失败 (第%d次): %s - %v",
			shortID, newCount, plugin.Name, err)
	}

	if err := s.updatePluginState(ctx, plugin.ID, updates); err != nil {
		logger.Sched("SYNC").Error("[%s] 更新插件同步失败状态失败: %v", shortID, err)
	}
}

func (s *Scheduler) handlePluginSyncResult(ctx context.Context, plugin model.Plugin, syncLog *model.PluginSyncLog, shortID string, nextSyncAt time.Time, duration time.Duration) {
	if syncLog.Status != "success" {
		s.handlePluginSyncStatusError(ctx, plugin, syncLog, shortID, nextSyncAt, duration)
		return
	}

	if err := s.updatePluginState(ctx, plugin.ID, map[string]interface{}{
		"consecutive_failures": 0,
		"pause_reason":         "",
		"next_sync_at":         nextSyncAt,
	}); err != nil {
		logger.Sched("SYNC").Error("[%s] 更新插件同步成功状态失败: %v", shortID, err)
	}
	if plugin.ConsecutiveFailures > 0 {
		logger.Sched("SYNC").Info("[%s] 同步成功: %s | 失败计数已重置 (之前: %d) | 耗时: %v",
			shortID, plugin.Name, plugin.ConsecutiveFailures, duration)
		return
	}

	logger.Sched("SYNC").Info("[%s] 同步完成: %s | 获取: %d条 | 新增: %d条 | 更新: %d条 | 失败: %d条 | 耗时: %v | 下次: %s",
		shortID,
		plugin.Name,
		syncLog.RecordsFetched,
		syncLog.RecordsNew,
		syncLog.RecordsUpdated,
		syncLog.RecordsFailed,
		duration,
		nextSyncAt.Format("15:04:05"),
	)
}

func (s *Scheduler) handlePluginSyncStatusError(ctx context.Context, plugin model.Plugin, syncLog *model.PluginSyncLog, shortID string, nextSyncAt time.Time, duration time.Duration) {
	newCount := plugin.ConsecutiveFailures + 1
	updates := map[string]interface{}{
		"consecutive_failures": newCount,
		"next_sync_at":         nextSyncAt,
	}

	if plugin.MaxFailures > 0 && newCount >= plugin.MaxFailures {
		updates["sync_enabled"] = false
		updates["next_sync_at"] = nil
		updates["pause_reason"] = fmt.Sprintf("连续失败 %d 次后自动暂停 (状态: %s)", newCount, syncLog.Status)
		logger.Sched("SYNC").Warn("[%s] ⚠ 连续失败 %d/%d 次，已自动暂停同步: %s",
			shortID, newCount, plugin.MaxFailures, plugin.Name)
	} else {
		logger.Sched("SYNC").Warn("[%s] 同步异常 (%d/%d): %s | 状态: %s | 获取: %d条 | 处理: %d条 | 失败: %d条 | 错误: %s | 耗时: %v",
			shortID,
			newCount,
			pluginFailureLimit(plugin),
			plugin.Name,
			syncLog.Status,
			syncLog.RecordsFetched,
			syncLog.RecordsProcessed,
			syncLog.RecordsFailed,
			cleanSchedulerError(syncLog.ErrorMessage),
			duration,
		)
	}

	if err := s.updatePluginState(ctx, plugin.ID, updates); err != nil {
		logger.Sched("SYNC").Error("[%s] 更新插件同步异常状态失败: %v", shortID, err)
	}
}

func (s *Scheduler) updatePluginSyncState(ctx context.Context, pluginID interface{}, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&model.Plugin{}).Where("id = ?", pluginID).Updates(updates).Error
}

func pluginFailureLimit(plugin model.Plugin) int {
	if plugin.MaxFailures > 0 {
		return plugin.MaxFailures
	}
	return 0
}

func cleanSchedulerError(message string) string {
	message = strings.ReplaceAll(message, "\n", " ")
	message = strings.ReplaceAll(message, "\r", "")
	if len(message) > 200 {
		return message[:200] + "..."
	}
	return message
}
