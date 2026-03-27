package git

import (
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	playbookSvc "github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	"gorm.io/gorm"
)

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo:         integrationrepo.NewGitRepositoryRepositoryWithDB(db),
		PlaybookRepo: integrationrepo.NewPlaybookRepositoryWithDB(db),
		ReposDir:     defaultReposDir(),
		PlaybookSvc: func() *playbookSvc.Service {
			return playbookSvc.NewServiceWithDB(db)
		},
		Lifecycle: newAsyncLifecycle(),
	}
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}
