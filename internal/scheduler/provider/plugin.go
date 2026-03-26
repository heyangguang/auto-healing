package provider

import (
	"context"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	pluginService "github.com/company/auto-healing/internal/service/plugin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Scheduler 定时调度器
type Scheduler struct {
	pluginSvc               *pluginService.Service
	cmdbSvc                 *pluginService.CMDBService
	db                      *gorm.DB
	interval                time.Duration
	lifecycle               *schedulerLifecycle
	inFlight                *inFlightSet
	now                     func() time.Time
	running                 bool
	mu                      sync.Mutex
	loadPluginsNeedSync     func(context.Context) ([]model.Plugin, error)
	checkExpiredMaintenance func(context.Context) (int, error)
	runPluginSync           func(context.Context, model.Plugin)
	triggerPluginSync       func(context.Context, uuid.UUID) (*model.PluginSyncLog, error)
	updatePluginState       func(context.Context, interface{}, map[string]interface{}) error
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	s := &Scheduler{
		pluginSvc: pluginService.NewService(),
		cmdbSvc:   pluginService.NewCMDBService(),
		db:        database.DB,
		interval:  30 * time.Second,
		inFlight:  newInFlightSet(),
		now:       time.Now,
	}
	s.loadPluginsNeedSync = s.getPluginsNeedSync
	s.checkExpiredMaintenance = s.cmdbSvc.CheckExpiredMaintenance
	s.runPluginSync = s.syncPlugin
	s.triggerPluginSync = s.pluginSvc.TriggerSyncSync
	s.updatePluginState = s.updatePluginSyncState
	return s
}

// Start 启动调度器
func (s *Scheduler) Start() {
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
	s.mu.Unlock()

	lifecycle.Stop()
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

	lifecycle := s.lifecycleSnapshot()
	for _, plugin := range plugins {
		s.dispatchPluginSync(lifecycle, plugin)
	}
}

func (s *Scheduler) dispatchPluginSync(lifecycle *schedulerLifecycle, plugin model.Plugin) {
	if lifecycle == nil {
		return
	}
	if !s.inFlight.Start(plugin.ID) {
		return
	}
	p := plugin
	started := lifecycle.Go(func(rootCtx context.Context) {
		defer s.inFlight.Finish(p.ID)
		defer func() {
			if rec := recover(); rec != nil {
				logger.Sched("SYNC").Error("[%s] syncPlugin panic: %v", p.ID.String()[:8], rec)
			}
		}()
		s.runPluginSync(withTenantContext(rootCtx, p.TenantID), p)
	})
	if !started {
		s.inFlight.Finish(p.ID)
	}
}

func (s *Scheduler) lifecycleSnapshot() *schedulerLifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lifecycle
}

// getPluginsNeedSync 获取需要同步的插件列表
func (s *Scheduler) getPluginsNeedSync(ctx context.Context) ([]model.Plugin, error) {
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
	return !lastSyncStillCoolingDown(plugin.LastSyncAt, interval, now)
}
