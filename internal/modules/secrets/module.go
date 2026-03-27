package secrets

import (
	secretshttp "github.com/company/auto-healing/internal/modules/secrets/httpapi"
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
)

// Module 聚合 secrets 域处理器构造。
type Module struct {
	Secrets *secretshttp.SecretsHandler
}

// New 创建 secrets 域模块。
func New() *Module {
	service := secretsSvc.NewServiceWithDeps(secretsSvc.ServiceDeps{
		Repo: secretsrepo.NewSecretsSourceRepository(),
	})
	return &Module{
		Secrets: secretshttp.NewSecretsHandlerWithDeps(secretshttp.SecretsHandlerDeps{
			Service: service,
		}),
	}
}
