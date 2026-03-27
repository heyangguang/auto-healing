package httpapi

import secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"

// NewSecretsHandler 保留兼容零参构造，生产路径应使用显式 deps。
func NewSecretsHandler() *SecretsHandler {
	return NewSecretsHandlerWithDeps(SecretsHandlerDeps{
		Service: secretsSvc.NewService(),
	})
}
