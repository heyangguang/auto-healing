// Package scheduler 调度器模块
//
// 本包提供各种调度器的统一接口和入口。
// 具体调度器实现在 provider/ 子目录中。
//
// 使用示例:
//
//	manager := scheduler.NewManager()
//	manager.Start()
//	defer manager.Stop()
package scheduler

import (
	"sync"

	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/scheduler/provider"
)

// Manager 调度器管理器
type Manager struct {
	pluginScheduler       *provider.Scheduler
	executionScheduler    *provider.ExecutionScheduler
	gitScheduler          *provider.GitScheduler
	notificationScheduler *provider.NotificationRetryScheduler
	blacklistScheduler    *provider.BlacklistExemptionScheduler
	mu                    sync.Mutex
	running               bool
}

// NewManager 创建调度器管理器
func NewManager() *Manager {
	return &Manager{
		pluginScheduler:       provider.NewScheduler(),
		executionScheduler:    provider.NewExecutionScheduler(),
		gitScheduler:          provider.NewGitScheduler(),
		notificationScheduler: provider.NewNotificationRetryScheduler(),
		blacklistScheduler:    provider.NewBlacklistExemptionScheduler(),
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
	Scheduler                   = provider.Scheduler
	ExecutionScheduler          = provider.ExecutionScheduler
	GitScheduler                = provider.GitScheduler
	NotificationRetryScheduler  = provider.NotificationRetryScheduler
	BlacklistExemptionScheduler = provider.BlacklistExemptionScheduler
)

// NewScheduler 创建插件调度器（向后兼容）
func NewScheduler() *provider.Scheduler {
	return provider.NewScheduler()
}

// NewExecutionScheduler 创建执行调度器（向后兼容）
func NewExecutionScheduler() *provider.ExecutionScheduler {
	return provider.NewExecutionScheduler()
}

// NewGitScheduler 创建 Git 调度器（向后兼容）
func NewGitScheduler() *provider.GitScheduler {
	return provider.NewGitScheduler()
}

// NewNotificationRetryScheduler 创建通知重试调度器（向后兼容）
func NewNotificationRetryScheduler() *provider.NotificationRetryScheduler {
	return provider.NewNotificationRetryScheduler()
}

// NewBlacklistExemptionScheduler 创建黑名单豁免过期调度器（向后兼容）
func NewBlacklistExemptionScheduler() *provider.BlacklistExemptionScheduler {
	return provider.NewBlacklistExemptionScheduler()
}
