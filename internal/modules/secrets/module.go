package secrets

import (
	"github.com/company/auto-healing/internal/handler"
	secretsSvc "github.com/company/auto-healing/internal/service/secrets"
)

// Module 聚合 secrets 域处理器构造。
type Module struct {
	Secrets *handler.SecretsHandler
}

// New 创建 secrets 域模块。
func New() *Module {
	return &Module{
		Secrets: handler.NewSecretsHandlerWithDeps(handler.SecretsHandlerDeps{
			Service: secretsSvc.NewService(),
		}),
	}
}
