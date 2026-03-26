package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	pluginService "github.com/company/auto-healing/internal/service/plugin"
	"gorm.io/gorm"
)

const pluginClaimLease = 40 * time.Minute

// Scheduler 定时调度器
type Scheduler struct {
	pluginSvc               *pluginService.Service
	cmdbSvc                 *pluginService.CMDBService
	db                      *gorm.DB
	interval                time.Duration
	lifecycle               *schedulerLifecycle
	running                 bool
	mu                      sync.Mutex
	loadPluginsNeedSync     func(context.Context) ([]model.Plugin, error)
	checkExpiredMaintenance func(context.Context) (int, error)
	runPluginSync           func(context.Context, model.Plugin)
	updateSyncState         func(context.Context, interface{}, map[string]interface{}) error
	claimPluginSync         func(context.Context, model.Plugin) (bool, error)
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	s := &Scheduler{
		pluginSvc: pluginService.NewService(),
		cmdbSvc:   pluginService.NewCMDBService(),
		db:        database.DB,
		interval:  30 * time.Second,
	}
	s.loadPluginsNeedSync = s.getPluginsNeedSync
	s.checkExpiredMaintenance = s.cmdbSvc.CheckExpiredMaintenance
	s.runPluginSync = s.syncPlugin
	s.updateSyncState = s.updatePluginSyncState
	s.claimPluginSync = s.claimPlugin
	return s
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.lifecycle = newSchedulerLifecycle()
	s.mu.Unlock()

	s.lifecycleGo(s.run)
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
	lifecycle := s.lifecycle
	s.lifecycle = nil
	s.mu.Unlock()

	if lifecycle != nil {
		lifecycle.Stop()
	}
	logger.Sched("SYNC").Info("插件同步调度器已停止")
}

// run 调度器主循环
func (s *Scheduler) run(ctx context.Context) {
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

// checkAndSync 检查并执行需要同步的插件
func (s *Scheduler) checkAndSync(ctx context.Context) {
	if count, err := s.checkExpiredMaintenance(ctx); err != nil {
		logger.Sched("SYNC").Warn("检查维护到期失败: %v", err)
	} else if count > 0 {
		logger.Sched("SYNC").Info("自动恢复 %d 个维护到期的配置项", count)
	}

	plugins, err := s.loadPluginsNeedSync(ctx)
	if err != nil {
		logger.Sched("SYNC").Error("查询待同步插件失败: %v", err)
		return
	}

	if len(plugins) == 0 {
		return
	}

	logger.Sched("SYNC").Info("发现 %d 个插件需要同步", len(plugins))

	for _, plugin := range plugins {
		claimed, err := s.claimPluginSync(ctx, plugin)
		if err != nil {
			logger.Sched("SYNC").Error("认领插件同步失败: %s (%s) - %v", plugin.Name, plugin.ID.String()[:8], err)
			continue
		}
		if !claimed {
			continue
		}
		if !s.dispatchPluginSync(plugin) {
			s.rollbackPluginClaim(ctx, plugin)
		}
	}
}

func (s *Scheduler) dispatchPluginSync(plugin model.Plugin) bool {
	p := plugin
	return s.lifecycleGo(func(rootCtx context.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				panicErr := fmt.Errorf("panic: %v", rec)
				shortID := p.ID.String()[:8]
				logger.Sched("SYNC").Error("[%s] syncPlugin panic: %v", shortID, rec)
				s.handlePluginSyncError(withTenantContext(rootCtx, p.TenantID), p, shortID, time.Now().Add(time.Duration(p.SyncIntervalMinutes)*time.Minute), panicErr)
			}
		}()
		s.runPluginSync(withTenantContext(rootCtx, p.TenantID), p)
	})
}

func (s *Scheduler) lifecycleGo(fn func(context.Context)) bool {
	s.mu.Lock()
	lifecycle := s.lifecycle
	running := s.running
	s.mu.Unlock()
	if !running || lifecycle == nil {
		return false
	}
	return lifecycle.Go(fn)
}

func (s *Scheduler) claimPlugin(ctx context.Context, plugin model.Plugin) (bool, error) {
	if plugin.NextSyncAt == nil {
		return false, nil
	}
	now := time.Now()
	interval := time.Duration(plugin.SyncIntervalMinutes) * time.Minute
	nextSyncAt := now.Add(maxDuration(interval, pluginClaimLease))
	result := s.db.WithContext(ctx).
		Model(&model.Plugin{}).
		Where("id = ? AND sync_enabled = ? AND status = ? AND next_sync_at IS NOT NULL AND next_sync_at <= ?", plugin.ID, true, "active", now).
		Update("next_sync_at", nextSyncAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (s *Scheduler) rollbackPluginClaim(ctx context.Context, plugin model.Plugin) {
	if plugin.NextSyncAt == nil {
		return
	}
	if err := s.updateSyncState(ctx, plugin.ID, map[string]interface{}{
		"next_sync_at": plugin.NextSyncAt,
	}); err != nil {
		logger.Sched("SYNC").Warn("回滚插件认领失败: %s (%s) - %v", plugin.Name, plugin.ID.String()[:8], err)
	}
}

// getPluginsNeedSync 获取需要同步的插件列表
func (s *Scheduler) getPluginsNeedSync(ctx context.Context) ([]model.Plugin, error) {
	var plugins []model.Plugin
	err := s.db.WithContext(ctx).
		Where("sync_enabled = ?", true).
		Where("status = ?", "active").
		Where("next_sync_at IS NOT NULL").
		Where("next_sync_at <= ?", time.Now()).
		Find(&plugins).Error

	return plugins, err
}
