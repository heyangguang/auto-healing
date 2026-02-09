package secrets

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/secrets"
	"github.com/google/uuid"
)

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

// CreateSource 创建密钥源
func (s *Service) CreateSource(ctx context.Context, source *model.SecretsSource) (*model.SecretsSource, error) {
	// 验证配置
	if _, err := secrets.NewProvider(source); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	if err := s.repo.Create(ctx, source); err != nil {
		return nil, err
	}

	return source, nil
}

// GetSource 获取密钥源
func (s *Service) GetSource(ctx context.Context, id uuid.UUID) (*model.SecretsSource, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdateSource 更新密钥源
func (s *Service) UpdateSource(ctx context.Context, id uuid.UUID, config model.JSON, isDefault *bool, priority *int, status string) (*model.SecretsSource, error) {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if config != nil {
		source.Config = config
	}
	if isDefault != nil {
		source.IsDefault = *isDefault
	}
	if priority != nil {
		source.Priority = *priority
	}
	if status != "" {
		source.Status = status
	}

	// 验证新配置
	if _, err := secrets.NewProvider(source); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	if err := s.repo.Update(ctx, source); err != nil {
		return nil, err
	}

	return source, nil
}

// DeleteSource 删除密钥源
func (s *Service) DeleteSource(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ListSources 列出密钥源
func (s *Service) ListSources(ctx context.Context, sourceType, status string, isDefault *bool) ([]model.SecretsSource, error) {
	return s.repo.List(ctx, sourceType, status, isDefault)
}

// TestConnection 测试连接（更新 last_test_at）
func (s *Service) TestConnection(ctx context.Context, id uuid.UUID) error {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	provider, err := secrets.NewProvider(source)
	if err != nil {
		return err
	}

	testErr := provider.TestConnection(ctx)

	// 更新最后测试时间
	s.repo.UpdateTestTime(ctx, id)

	return testErr
}

// TestQuery 测试能否获取指定主机的凭据（仅作查询工具，不影响启用）
func (s *Service) TestQuery(ctx context.Context, id uuid.UUID, hostname, ipAddress string) (*model.Secret, error) {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("密钥源不存在")
	}

	provider, err := secrets.NewProvider(source)
	if err != nil {
		return nil, fmt.Errorf("创建提供者失败: %w", err)
	}

	query := model.SecretQuery{
		Hostname:  hostname,
		IPAddress: ipAddress,
		AuthType:  source.AuthType,
	}

	secret, err := provider.GetSecret(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("获取凭据失败: %w", err)
	}

	logger.Auth("SECRET").Info("测试成功: %s | 密钥源: %s", hostname, source.Name)
	return secret, nil
}

// QuerySecret 查询密钥（可选指定 source_id，不指定则使用默认密钥源）
func (s *Service) QuerySecret(ctx context.Context, query model.SecretQuery) (*model.Secret, error) {
	var source *model.SecretsSource
	var err error

	if query.SourceID != "" {
		// 指定了 source_id，使用指定的密钥源
		sourceID, parseErr := uuid.Parse(query.SourceID)
		if parseErr != nil {
			return nil, fmt.Errorf("无效的密钥源ID: %v", parseErr)
		}
		source, err = s.repo.GetByID(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("密钥源不存在: %v", err)
		}
	} else {
		// 未指定 source_id，使用默认密钥源
		source, err = s.repo.GetDefault(ctx)
		if err != nil {
			return nil, fmt.Errorf("未找到可用的默认密钥源，请先创建密钥源或指定 source_id")
		}
		logger.Auth("SECRET").Info("使用默认密钥源: %s (ID: %s)", source.Name, source.ID)
	}

	provider, err := secrets.NewProvider(source)
	if err != nil {
		return nil, err
	}

	secret, err := provider.GetSecret(ctx, query)
	if err != nil {
		logger.Auth("SECRET").Error("失败: %s | 密钥源: %s | 错误: %v", query.Hostname, source.Name, err)
		return nil, err
	}

	logger.Auth("SECRET").Info("完成: %s | 源类型: %s | 来源: %s | 认证类型: %s",
		query.Hostname, source.Type, source.Name, secret.AuthType)
	return secret, nil
}

// Enable 启用密钥源（先测试连接，通过后才启用）
func (s *Service) Enable(ctx context.Context, id uuid.UUID) error {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("密钥源不存在")
	}

	if source.Status == "active" {
		return fmt.Errorf("密钥源已经是启用状态")
	}

	// 先测试连接
	provider, err := secrets.NewProvider(source)
	if err != nil {
		return fmt.Errorf("配置错误: %w", err)
	}

	if err := provider.TestConnection(ctx); err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}

	return s.repo.UpdateStatus(ctx, id, "active")
}

// Disable 禁用密钥源
func (s *Service) Disable(ctx context.Context, id uuid.UUID) error {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("密钥源不存在")
	}

	if source.Status == "inactive" {
		return fmt.Errorf("密钥源已经是禁用状态")
	}

	return s.repo.UpdateStatus(ctx, id, "inactive")
}
