package secrets

import (
	secretshttp "github.com/company/auto-healing/internal/modules/secrets/httpapi"
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	"gorm.io/gorm"
)

// Module 聚合 secrets 域处理器构造。
type Module struct {
	Secrets *secretshttp.SecretsHandler
}

type ModuleDeps struct {
	Service *secretsSvc.Service
}

func DefaultModuleDepsWithDB(db *gorm.DB) ModuleDeps {
	return ModuleDeps{
		Service: secretsSvc.NewServiceWithDB(db),
	}
}

func NewWithDB(db *gorm.DB) *Module {
	return NewWithDeps(DefaultModuleDepsWithDB(db))
}

func NewWithDeps(deps ModuleDeps) *Module {
	if deps.Service == nil {
		panic("secrets module requires service")
	}
	return &Module{
		Secrets: secretshttp.NewSecretsHandlerWithDeps(secretshttp.SecretsHandlerDeps{
			Service: deps.Service,
		}),
	}
}
