package secrets

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/secrets/provider"
)

var (
	ErrSecretNotFound          = provider.ErrSecretNotFound
	ErrProviderNotFound        = errors.New("密钥源不可用")
	ErrUnsupportedType         = errors.New("不支持的密钥源类型")
	ErrConnectionFailed        = provider.ErrConnectionFailed
	ErrProviderAuthFailed      = provider.ErrProviderAuthFailed
	ErrProviderRequestFailed   = provider.ErrProviderRequestFailed
	ErrProviderInvalidConfig   = provider.ErrProviderInvalidConfig
	ErrProviderInvalidResponse = provider.ErrProviderInvalidResponse
)

// Provider 密钥提供者接口
type Provider interface {
	// GetSecret 获取密钥
	GetSecret(ctx context.Context, query model.SecretQuery) (*model.Secret, error)

	// TestConnection 测试连接
	TestConnection(ctx context.Context) error

	// Name 获取提供者名称
	Name() string
}

// NewProvider 创建密钥提供者（工厂模式）
func NewProvider(source *model.SecretsSource) (Provider, error) {
	switch source.Type {
	case "file":
		return provider.NewFileProvider(source)
	case "vault":
		return provider.NewVaultProvider(source)
	case "webhook":
		return provider.NewWebhookProvider(source)
	default:
		return nil, ErrUnsupportedType
	}
}
