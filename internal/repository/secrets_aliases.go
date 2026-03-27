package repository

import (
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	"gorm.io/gorm"
)

type SecretsSourceRepository = secretsrepo.SecretsSourceRepository

func NewSecretsSourceRepository() *SecretsSourceRepository {
	return secretsrepo.NewSecretsSourceRepository()
}

func NewSecretsSourceRepositoryWithDB(db *gorm.DB) *SecretsSourceRepository {
	return secretsrepo.NewSecretsSourceRepositoryWithDB(db)
}
