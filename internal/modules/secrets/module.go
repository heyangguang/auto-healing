package secrets

import (
	secretshttp "github.com/company/auto-healing/internal/modules/secrets/httpapi"
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
)

// Module 聚合 secrets 域处理器构造。
type Module struct {
	Secrets *secretshttp.SecretsHandler
}

type ModuleDeps struct {
	Service *secretsSvc.Service
}

func DefaultModuleDeps() ModuleDeps {
	return ModuleDeps{
		Service: secretsSvc.NewServiceWithDeps(secretsSvc.DefaultServiceDeps()),
	}
}

// New 创建 secrets 域模块。
func New() *Module {
	return NewWithDeps(DefaultModuleDeps())
}

func NewWithDeps(deps ModuleDeps) *Module {
	service := deps.Service
	if service == nil {
		service = secretsSvc.NewServiceWithDeps(secretsSvc.DefaultServiceDeps())
	}
	return &Module{
		Secrets: secretshttp.NewSecretsHandlerWithDeps(secretshttp.SecretsHandlerDeps{
			Service: service,
		}),
	}
}
