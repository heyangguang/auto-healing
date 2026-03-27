package git

import (
	"github.com/company/auto-healing/internal/database"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	playbookSvc "github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	"gorm.io/gorm"
)

func DefaultServiceDeps() ServiceDeps {
	return DefaultServiceDepsWithDB(database.DB)
}

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

// NewService 保留兼容零参构造，生产路径应使用显式 deps。
func NewService() *Service {
	return NewServiceWithDB(database.DB)
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}
