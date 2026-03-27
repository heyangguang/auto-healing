package schedule

import (
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"gorm.io/gorm"
)

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo:     automationrepo.NewScheduleRepositoryWithDB(db),
		ExecRepo: automationrepo.NewExecutionRepositoryWithDB(db),
	}
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}
