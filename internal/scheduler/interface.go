// Package scheduler 调度器模块
//
// 本包提供各种调度器的统一接口和入口。
// 具体调度器实现在各业务域模块内。
//
// 使用示例:
//
//	manager := scheduler.NewManager()
//	manager.Start()
//	defer manager.Stop()
package scheduler

import (
	"sync"

	automationsched "github.com/company/auto-healing/internal/modules/automation/scheduler"
	engagementsched "github.com/company/auto-healing/internal/modules/engagement/scheduler"
	integrationsched "github.com/company/auto-healing/internal/modules/integrations/scheduler"
	opssched "github.com/company/auto-healing/internal/modules/ops/scheduler"
	"github.com/company/auto-healing/internal/pkg/logger"
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
	return &Manager{
		pluginScheduler:       integrationsched.NewPluginScheduler(),
		executionScheduler:    automationsched.NewExecutionScheduler(),
		gitScheduler:          integrationsched.NewGitScheduler(),
		notificationScheduler: engagementsched.NewNotificationRetryScheduler(),
		blacklistScheduler:    opssched.NewBlacklistExemptionScheduler(),
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
