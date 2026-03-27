package secrets

import secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"

// Service 密钥服务
type Service struct {
	repo *secretsrepo.SecretsSourceRepository
}

type ServiceDeps struct {
	Repo *secretsrepo.SecretsSourceRepository
}

func DefaultServiceDeps() ServiceDeps {
	return ServiceDeps{
		Repo: secretsrepo.NewSecretsSourceRepository(),
	}
}

// NewService 创建密钥服务
func NewService() *Service {
	return NewServiceWithDeps(DefaultServiceDeps())
}

func NewServiceWithDeps(deps ServiceDeps) *Service {
	return &Service{
		repo: deps.Repo,
	}
}
