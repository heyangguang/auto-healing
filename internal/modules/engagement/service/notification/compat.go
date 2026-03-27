package notification

import (
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"gorm.io/gorm"
)

// NewService 保留兼容入口；主实现统一走 WithDeps。
func NewService(db *gorm.DB, systemName, systemURL, systemVersion string) *Service {
	return newServiceWithRuntime(ConfiguredServiceDeps{
		Repo:            engagementrepo.NewNotificationRepository(db),
		HealingFlowRepo: automationrepo.NewHealingFlowRepositoryWithDB(db),
	}, systemName, systemURL, systemVersion)
}

// NewConfiguredService 保留兼容入口；主实现统一走 WithDeps。
func NewConfiguredService(db *gorm.DB) *Service {
	return NewConfiguredServiceWithDeps(ConfiguredServiceDeps{
		Repo:            engagementrepo.NewNotificationRepository(db),
		HealingFlowRepo: automationrepo.NewHealingFlowRepositoryWithDB(db),
	})
}
