package secrets

import (
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

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo: secretsrepo.NewSecretsSourceRepositoryWithDB(db),
	}
}

func NewServiceWithDeps(deps ServiceDeps) *Service {
	if deps.Repo == nil {
		panic("secrets service requires repo")
	}
	return &Service{
		repo: deps.Repo,
	}
}
