package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/integrations/model"
	pluginService "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	"github.com/company/auto-healing/internal/pkg/logger"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const pluginClaimLease = 40 * time.Minute

// PluginScheduler 插件同步调度器
type PluginScheduler struct {
	pluginSvc               *pluginService.Service
	cmdbSvc                 *pluginService.CMDBService
	db                      *gorm.DB
	interval                time.Duration
	lifecycle               *schedulerx.Lifecycle
	inFlight                *schedulerx.InFlightSet
	now                     func() time.Time
	running                 bool
	mu                      sync.Mutex
	loadPluginsNeedSync     func(context.Context) ([]model.Plugin, error)
	checkExpiredMaintenance func(context.Context) (int, error)
	runPluginSync           func(context.Context, model.Plugin)
	triggerPluginSync       func(context.Context, uuid.UUID) (*model.PluginSyncLog, error)
	updateSyncState         func(context.Context, interface{}, map[string]interface{}) error
	updatePluginState       func(context.Context, interface{}, map[string]interface{}) error
	claimPluginSync         func(context.Context, model.Plugin) (bool, error)
}

// NewPluginScheduler 创建调度器
func NewPluginScheduler() *PluginScheduler {
	s := &PluginScheduler{
		pluginSvc: pluginService.NewService(),
		cmdbSvc:   pluginService.NewCMDBService(),
		db:        database.DB,
		interval:  30 * time.Second,
		inFlight:  schedulerx.NewInFlightSet(),
		now:       time.Now,
	}
	s.loadPluginsNeedSync = s.getPluginsNeedSync
	s.checkExpiredMaintenance = s.cmdbSvc.CheckExpiredMaintenance
	s.runPluginSync = s.syncPlugin
	s.triggerPluginSync = s.pluginSvc.TriggerSyncSync
	s.updatePluginState = s.updatePluginSyncState
	s.claimPluginSync = s.claimPlugin
	return s
}

// Start 启动调度器
func (s *PluginScheduler) Start() {
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
	logger.Sched("SYNC").Info("插件同步调度器已启动 (检查间隔: %v)", s.interval)
}

// Stop 停止调度器
func (s *PluginScheduler) Stop() {
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
	logger.Sched("SYNC").Info("插件同步调度器已停止")
}

// run 调度器主循环
func (s *PluginScheduler) run(ctx context.Context) {
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
func (s *PluginScheduler) checkAndSync(ctx context.Context) {
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

	lifecycle := s.lifecycleSnapshot()
	for _, plugin := range plugins {
		claimed, err := s.claimPluginSync(ctx, plugin)
		if err != nil {
			logger.Sched("SYNC").Error("认领插件同步失败: %s (%s) - %v", plugin.Name, plugin.ID.String()[:8], err)
			continue
		}
		if !claimed {
			continue
		}
		if !s.dispatchPluginSync(lifecycle, plugin) {
			s.rollbackPluginClaim(ctx, plugin)
		}
	}
}

func (s *PluginScheduler) dispatchPluginSync(lifecycle *schedulerx.Lifecycle, plugin model.Plugin) bool {
	if lifecycle == nil {
		return false
	}
	if !s.inFlight.Start(plugin.ID) {
		return false
	}

	p := plugin
	started := lifecycle.Go(func(rootCtx context.Context) {
		defer s.inFlight.Finish(p.ID)
		defer func() {
			if rec := recover(); rec != nil {
				panicErr := fmt.Errorf("panic: %v", rec)
				shortID := p.ID.String()[:8]
				logger.Sched("SYNC").Error("[%s] syncPlugin panic: %v", shortID, rec)
				s.handlePluginSyncError(schedulerx.WithTenantContext(rootCtx, p.TenantID), p, shortID, s.now().Add(time.Duration(p.SyncIntervalMinutes)*time.Minute), panicErr)
			}
		}()
		s.runPluginSync(schedulerx.WithTenantContext(rootCtx, p.TenantID), p)
	})
	if !started {
		s.inFlight.Finish(p.ID)
	}
	return started
}

func (s *PluginScheduler) lifecycleSnapshot() *schedulerx.Lifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lifecycle
}

func (s *PluginScheduler) claimPlugin(ctx context.Context, plugin model.Plugin) (bool, error) {
	if plugin.NextSyncAt == nil {
		return false, nil
	}
	now := s.now()
	interval := time.Duration(plugin.SyncIntervalMinutes) * time.Minute
	nextSyncAt := now.Add(schedulerx.MaxDuration(interval, pluginClaimLease))
	result := s.db.WithContext(ctx).
		Model(&model.Plugin{}).
		Where("id = ? AND sync_enabled = ? AND status = ? AND next_sync_at IS NOT NULL AND next_sync_at <= ?", plugin.ID, true, "active", now).
		Update("next_sync_at", nextSyncAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (s *PluginScheduler) rollbackPluginClaim(ctx context.Context, plugin model.Plugin) {
	if plugin.NextSyncAt == nil {
		return
	}
	if err := s.persistPluginState(ctx, plugin.ID, map[string]interface{}{
		"next_sync_at": plugin.NextSyncAt,
	}); err != nil {
		logger.Sched("SYNC").Warn("回滚插件认领失败: %s (%s) - %v", plugin.Name, plugin.ID.String()[:8], err)
	}
}

// getPluginsNeedSync 获取需要同步的插件列表
func (s *PluginScheduler) getPluginsNeedSync(ctx context.Context) ([]model.Plugin, error) {
	var plugins []model.Plugin
	now := s.now()
	err := s.db.WithContext(ctx).
		Where("sync_enabled = ?", true).
		Where("status = ?", "active").
		Where("next_sync_at IS NOT NULL").
		Where("next_sync_at <= ?", now).
		Find(&plugins).Error

	return filterDuePlugins(plugins, now), err
}

func filterDuePlugins(plugins []model.Plugin, now time.Time) []model.Plugin {
	due := plugins[:0]
	for _, plugin := range plugins {
		if pluginSyncDue(plugin, now) {
			due = append(due, plugin)
		}
	}
	return due
}

func pluginSyncDue(plugin model.Plugin, now time.Time) bool {
	interval := time.Duration(plugin.SyncIntervalMinutes) * time.Minute
	return !schedulerx.LastSyncStillCoolingDown(plugin.LastSyncAt, interval, now)
}

func (s *PluginScheduler) persistPluginState(ctx context.Context, pluginID interface{}, updates map[string]interface{}) error {
	if s.updateSyncState != nil {
		return s.updateSyncState(ctx, pluginID, updates)
	}
	if s.updatePluginState != nil {
		return s.updatePluginState(ctx, pluginID, updates)
	}
	return nil
}
