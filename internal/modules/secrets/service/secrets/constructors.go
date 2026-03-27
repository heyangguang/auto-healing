package secrets

import (
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	"gorm.io/gorm"
)

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo: secretsrepo.NewSecretsSourceRepositoryWithDB(db),
	}
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}
