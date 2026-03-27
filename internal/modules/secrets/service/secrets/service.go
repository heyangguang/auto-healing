package secrets

import (
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
)

// Service 密钥服务
type Service struct {
	repo *secretsrepo.SecretsSourceRepository
}

type ServiceDeps struct {
	Repo *secretsrepo.SecretsSourceRepository
}

func NewServiceWithDeps(deps ServiceDeps) *Service {
	if deps.Repo == nil {
		panic("secrets service requires repo")
	}
	return &Service{
		repo: deps.Repo,
	}
}
