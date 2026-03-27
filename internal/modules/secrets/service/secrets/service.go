package secrets

import secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"

// Service 密钥服务
type Service struct {
	repo *secretsrepo.SecretsSourceRepository
}

// NewService 创建密钥服务
func NewService() *Service {
	return &Service{
		repo: secretsrepo.NewSecretsSourceRepository(),
	}
}
