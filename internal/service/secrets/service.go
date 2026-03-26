package secrets

import "github.com/company/auto-healing/internal/repository"

// Service 密钥服务
type Service struct {
	repo *repository.SecretsSourceRepository
}

// NewService 创建密钥服务
func NewService() *Service {
	return &Service{
		repo: repository.NewSecretsSourceRepository(),
	}
}
