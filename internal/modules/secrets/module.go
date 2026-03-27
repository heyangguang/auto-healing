package secrets

import (
	"github.com/company/auto-healing/internal/database"
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

func DefaultModuleDeps() ModuleDeps {
	return DefaultModuleDepsWithDB(database.DB)
}

func DefaultModuleDepsWithDB(db *gorm.DB) ModuleDeps {
	return ModuleDeps{
		Service: secretsSvc.NewServiceWithDB(db),
	}
}

// New 创建 secrets 域模块。
func New() *Module {
	return NewWithDB(database.DB)
}

func NewWithDB(db *gorm.DB) *Module {
	return NewWithDeps(DefaultModuleDepsWithDB(db))
}

func NewWithDeps(deps ModuleDeps) *Module {
	service := deps.Service
	if service == nil {
		service = secretsSvc.NewServiceWithDB(database.DB)
	}
	return &Module{
		Secrets: secretshttp.NewSecretsHandlerWithDeps(secretshttp.SecretsHandlerDeps{
			Service: service,
		}),
	}
}
