// Package runtime 应用运行时装配模块
//
// 本包提供各种调度器的统一接口和入口。
// 具体调度器实现在各业务域模块内。
//
// 使用示例:
//
//	manager := runtime.NewManager()
//	manager.Start()
//	defer manager.Stop()
package runtime

import (
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	automationsched "github.com/company/auto-healing/internal/modules/automation/scheduler"
	executionSvc "github.com/company/auto-healing/internal/modules/automation/service/execution"
	scheduleSvc "github.com/company/auto-healing/internal/modules/automation/service/schedule"
	engagementsched "github.com/company/auto-healing/internal/modules/engagement/scheduler"
	notificationSvc "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	integrationsched "github.com/company/auto-healing/internal/modules/integrations/scheduler"
	gitSvc "github.com/company/auto-healing/internal/modules/integrations/service/git"
	playbookSvc "github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	pluginSvc "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opssched "github.com/company/auto-healing/internal/modules/ops/scheduler"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	"github.com/company/auto-healing/internal/pkg/logger"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
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

// NewManager 创建调度器管理器
func NewManager() *Manager {
	executionRepo := automationrepo.NewExecutionRepository()
	scheduleRepo := automationrepo.NewScheduleRepository()
	flowRepo := automationrepo.NewHealingFlowRepository()
	incidentRepository := incidentrepo.NewIncidentRepository()
	gitRepoRepo := integrationrepo.NewGitRepositoryRepository()
	playbookRepo := integrationrepo.NewPlaybookRepository()
	secretRepo := secretsrepo.NewSecretsSourceRepository()
	cmdbRepo := cmdbrepo.NewCMDBItemRepository()
	notifSvc := notificationSvc.NewConfiguredService(database.DB)
	execSvc := executionSvc.NewServiceWithDeps(executionSvc.ServiceDeps{
		Repo:             executionRepo,
		GitRepo:          gitRepoRepo,
		SecretsRepo:      secretRepo,
		CMDBRepo:         cmdbRepo,
		HealingFlowRepo:  flowRepo,
		WorkspaceManager: ansible.NewWorkspaceManager(),
		LocalExecutor:    ansible.NewLocalExecutor(),
		DockerExecutor:   ansible.NewDockerExecutor(),
		NotificationSvc:  notifSvc,
		BlacklistSvc:     opsservice.NewCommandBlacklistService(),
		ExemptionSvc:     opsservice.NewBlacklistExemptionService(),
	})
	schedSvc := scheduleSvc.NewServiceWithDeps(scheduleSvc.ServiceDeps{
		Repo:     scheduleRepo,
		ExecRepo: executionRepo,
	})
	playbookFactory := func() *playbookSvc.Service {
		return playbookSvc.NewServiceWithDeps(playbookSvc.ServiceDeps{
			Repo:          playbookRepo,
			GitRepo:       gitRepoRepo,
			ExecutionRepo: executionRepo,
		})
	}
	gitService := gitSvc.NewServiceWithDeps(gitSvc.ServiceDeps{
		Repo:         gitRepoRepo,
		PlaybookRepo: playbookRepo,
		PlaybookSvc:  playbookFactory,
	})
	httpClient := pluginSvc.NewHTTPClient()
	pluginService := pluginSvc.NewServiceWithDeps(pluginSvc.ServiceDeps{
		PluginRepo:   integrationrepo.NewPluginRepository(),
		SyncLogRepo:  integrationrepo.NewPluginSyncLogRepository(),
		CMDBRepo:     cmdbRepo,
		IncidentRepo: incidentRepository,
		HTTPClient:   httpClient,
	})
	cmdbService := pluginSvc.NewCMDBServiceWithDeps(pluginSvc.CMDBServiceDeps{CMDBRepo: cmdbRepo})
	blacklistExemptionService := opsservice.NewBlacklistExemptionServiceWithDeps(opsservice.BlacklistExemptionServiceDeps{
		Repo: opsrepo.NewBlacklistExemptionRepository(database.DB),
	})
	return &Manager{
		pluginScheduler: integrationsched.NewPluginSchedulerWithDeps(integrationsched.PluginSchedulerDeps{
			PluginService: pluginService,
			CMDBService:   cmdbService,
			DB:            database.DB,
			Interval:      30 * time.Second,
			InFlight:      schedulerx.NewInFlightSet(),
			Now:           time.Now,
		}),
		executionScheduler: automationsched.NewExecutionSchedulerWithDeps(automationsched.ExecutionSchedulerDeps{
			ExecutionService: execSvc,
			ScheduleService:  schedSvc,
			ScheduleRepo:     scheduleRepo,
			DB:               database.DB,
			Interval:         30 * time.Second,
			InFlight:         schedulerx.NewInFlightSet(),
			Sem:              make(chan struct{}, 8),
		}),
		gitScheduler: integrationsched.NewGitSchedulerWithDeps(integrationsched.GitSchedulerDeps{
			GitService: gitService,
			DB:         database.DB,
			Interval:   60 * time.Second,
			InFlight:   schedulerx.NewInFlightSet(),
			Now:        time.Now,
		}),
		notificationScheduler: engagementsched.NewNotificationRetryScheduler(),
		blacklistScheduler: opssched.NewBlacklistExemptionSchedulerWithDeps(opssched.BlacklistExemptionSchedulerDeps{
			Service:    blacklistExemptionService,
			Lifecycle:  schedulerx.NewLifecycle(),
			ExpireFunc: blacklistExemptionService.ExpireOverdue,
		}),
	}
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

// NewScheduler 创建插件调度器（向后兼容）
func NewScheduler() *integrationsched.PluginScheduler {
	return integrationsched.NewPluginScheduler()
}

// NewExecutionScheduler 创建执行调度器（向后兼容）
func NewExecutionScheduler() *automationsched.ExecutionScheduler {
	return automationsched.NewExecutionScheduler()
}

// NewGitScheduler 创建 Git 调度器（向后兼容）
func NewGitScheduler() *integrationsched.GitScheduler {
	return integrationsched.NewGitScheduler()
}

// NewNotificationRetryScheduler 创建通知重试调度器（向后兼容）
func NewNotificationRetryScheduler() *engagementsched.NotificationRetryScheduler {
	return engagementsched.NewNotificationRetryScheduler()
}

// NewBlacklistExemptionScheduler 创建黑名单豁免过期调度器（向后兼容）
func NewBlacklistExemptionScheduler() *opssched.BlacklistExemptionScheduler {
	return opssched.NewBlacklistExemptionScheduler()
}
