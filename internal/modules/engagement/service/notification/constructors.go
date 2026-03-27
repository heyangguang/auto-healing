package notification

import (
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"gorm.io/gorm"
)

func NewService(db *gorm.DB, systemName, systemURL, systemVersion string) *Service {
	return newServiceWithRuntime(ConfiguredServiceDeps{
		Repo:            engagementrepo.NewNotificationRepository(db),
		HealingFlowRepo: automationrepo.NewHealingFlowRepositoryWithDB(db),
	}, systemName, systemURL, systemVersion)
}

func NewConfiguredService(db *gorm.DB) *Service {
	return NewConfiguredServiceWithDeps(ConfiguredServiceDeps{
		Repo:            engagementrepo.NewNotificationRepository(db),
		HealingFlowRepo: automationrepo.NewHealingFlowRepositoryWithDB(db),
	})
}
