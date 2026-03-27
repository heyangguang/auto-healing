package secrets

import (
	"github.com/company/auto-healing/internal/database"
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	"gorm.io/gorm"
)

// Service 密钥服务
type Service struct {
	repo *secretsrepo.SecretsSourceRepository
}

type ServiceDeps struct {
	Repo *secretsrepo.SecretsSourceRepository
}

func DefaultServiceDeps() ServiceDeps {
	return DefaultServiceDepsWithDB(database.DB)
}

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo: secretsrepo.NewSecretsSourceRepositoryWithDB(db),
	}
}

// NewService 创建密钥服务
func NewService() *Service {
	return NewServiceWithDB(database.DB)
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}

func NewServiceWithDeps(deps ServiceDeps) *Service {
	return &Service{
		repo: deps.Repo,
	}
}
