// Package runtime 应用运行时装配模块
//
// 本包提供各种调度器的统一接口和入口。
// 具体调度器实现在各业务域模块内。
//
// 使用示例:
//
//	manager := runtime.NewManagerWithDeps(runtime.ManagerDeps{DB: db})
//	manager.Start()
//	defer manager.Stop()
package runtime

import (
	"sync"

	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	automationsched "github.com/company/auto-healing/internal/modules/automation/scheduler"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	engagementsched "github.com/company/auto-healing/internal/modules/engagement/scheduler"
	notificationSvc "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	integrationsched "github.com/company/auto-healing/internal/modules/integrations/scheduler"
	opssched "github.com/company/auto-healing/internal/modules/ops/scheduler"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/company/auto-healing/internal/pkg/logger"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
	"gorm.io/gorm"
)

// Manager 调度器管理器
type Manager struct {
	pluginScheduler       *integrationsched.PluginScheduler
	executionScheduler    *automationsched.ExecutionScheduler
	gitScheduler          *integrationsched.GitScheduler
	notificationScheduler *engagementsched.NotificationRetryScheduler
	blacklistScheduler    *opssched.BlacklistExemptionScheduler
	mu                    sync.Mutex
	running               bool
}

type ManagerDeps struct {
	DB *gorm.DB
}

func NewManagerWithDeps(deps ManagerDeps) *Manager {
	db := requireManagerDB(deps.DB)
	notifSvc := notificationSvc.NewConfiguredServiceWithDeps(notificationSvc.ConfiguredServiceDeps{
		Repo:            engagementrepo.NewNotificationRepository(db),
		HealingFlowRepo: automationrepo.NewHealingFlowRepositoryWithDB(db),
	})
	blacklistExemptionService := opsservice.NewBlacklistExemptionServiceWithDB(db)
	return &Manager{
		pluginScheduler:    integrationsched.NewPluginSchedulerWithDB(db),
		executionScheduler: automationsched.NewExecutionSchedulerWithDB(db),
		gitScheduler:       integrationsched.NewGitSchedulerWithDB(db),
		notificationScheduler: engagementsched.NewNotificationRetrySchedulerWithDeps(engagementsched.NotificationRetrySchedulerDeps{
			NotificationService: notifSvc,
		}),
		blacklistScheduler: opssched.NewBlacklistExemptionSchedulerWithDeps(opssched.BlacklistExemptionSchedulerDeps{
			Service:    blacklistExemptionService,
			Lifecycle:  schedulerx.NewLifecycle(),
			ExpireFunc: blacklistExemptionService.ExpireOverdue,
		}),
	}
}

func requireManagerDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("runtime manager requires explicit db")
	}
	return db
}

// Start 启动所有调度器
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}
	m.running = true

	m.pluginScheduler.Start()
	m.executionScheduler.Start()
	m.gitScheduler.Start()
	m.notificationScheduler.Start()
	m.blacklistScheduler.Start()

	logger.Info("调度器管理器已启动")
}

// Stop 停止所有调度器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	m.running = false

	m.pluginScheduler.Stop()
	m.executionScheduler.Stop()
	m.gitScheduler.Stop()
	m.notificationScheduler.Stop()
	m.blacklistScheduler.Stop()

	logger.Info("调度器管理器已停止")
}

// 类型别名，保持向后兼容
type (
	Scheduler                   = integrationsched.PluginScheduler
	ExecutionScheduler          = automationsched.ExecutionScheduler
	GitScheduler                = integrationsched.GitScheduler
	NotificationRetryScheduler  = engagementsched.NotificationRetryScheduler
	BlacklistExemptionScheduler = opssched.BlacklistExemptionScheduler
)
