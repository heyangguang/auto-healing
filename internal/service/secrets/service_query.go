package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/secrets"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) TestQuery(ctx context.Context, id uuid.UUID, hostname, ipAddress string) (*model.Secret, error) {
	source, err := s.GetSource(ctx, id)
	if err != nil {
		return nil, err
	}
	provider, err := secrets.NewProvider(source)
	if err != nil {
		return nil, fmt.Errorf("创建提供者失败: %w", err)
	}
	secret, err := provider.GetSecret(ctx, model.SecretQuery{Hostname: hostname, IPAddress: ipAddress, AuthType: source.AuthType})
	if err != nil {
		return nil, fmt.Errorf("获取凭据失败: %w", err)
	}
	logger.Auth("SECRET").Info("测试成功: %s | 密钥源: %s", hostname, source.Name)
	return secret, nil
}

func (s *Service) QuerySecret(ctx context.Context, query model.SecretQuery) (*model.Secret, error) {
	source, err := s.resolveSecretsSource(ctx, query.SourceID)
	if err != nil {
		return nil, err
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
	logger.Auth("SECRET").Info("完成: %s | 源类型: %s | 来源: %s | 认证类型: %s", query.Hostname, source.Type, source.Name, secret.AuthType)
	return secret, nil
}

func (s *Service) resolveSecretsSource(ctx context.Context, sourceID string) (*model.SecretsSource, error) {
	if sourceID == "" {
		source, err := s.repo.GetDefault(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("%w: 未找到可用的默认密钥源，请先创建密钥源或指定 source_id", ErrDefaultSecretsSourceUnavailable)
			}
			return nil, err
		}
		logger.Auth("SECRET").Info("使用默认密钥源: %s (ID: %s)", source.Name, source.ID)
		return source, nil
	}

	parsedID, err := uuid.Parse(sourceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSecretsSourceInvalidID, err)
	}
	source, err := s.repo.GetByID(ctx, parsedID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrSecretsSourceNotFound, parsedID)
		}
		return nil, err
	}
	if source.Status != "active" {
		return nil, fmt.Errorf("%w: %s", ErrSecretsSourceInactive, source.Name)
	}
	return source, nil
}
