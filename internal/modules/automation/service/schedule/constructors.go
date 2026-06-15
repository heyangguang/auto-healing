package schedule

import (
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	"gorm.io/gorm"
)

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo:     automationrepo.NewScheduleRepositoryWithDB(db),
		ExecRepo: automationrepo.NewExecutionRepositoryWithDB(db),
		CMDBRepo: cmdbrepo.NewCMDBItemRepositoryWithDB(db),
	}
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}
