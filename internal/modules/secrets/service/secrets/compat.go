package secrets

import (
	"github.com/company/auto-healing/internal/database"
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	"gorm.io/gorm"
)

func DefaultServiceDeps() ServiceDeps {
	return DefaultServiceDepsWithDB(database.DB)
}

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo: secretsrepo.NewSecretsSourceRepositoryWithDB(db),
	}
}

// NewService 保留兼容零参构造，生产路径应使用显式 deps。
func NewService() *Service {
	return NewServiceWithDB(database.DB)
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}
