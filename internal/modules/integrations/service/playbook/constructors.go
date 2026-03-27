package playbook

import (
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"gorm.io/gorm"
)

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo:          integrationrepo.NewPlaybookRepositoryWithDB(db),
		GitRepo:       integrationrepo.NewGitRepositoryRepositoryWithDB(db),
		ExecutionRepo: automationrepo.NewExecutionRepositoryWithDB(db),
	}
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}
