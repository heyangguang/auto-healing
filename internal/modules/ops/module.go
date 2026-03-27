package ops

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/handler"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/company/auto-healing/internal/repository"
)

// Module 聚合 ops 域处理器构造。
type Module struct {
	Audit              *handler.AuditHandler
	PlatformAudit      *handler.PlatformAuditHandler
	PlatformSettings   *handler.PlatformSettingsHandler
	Dictionary         *handler.DictionaryHandler
	CommandBlacklist   *handler.CommandBlacklistHandler
	BlacklistExemption *handler.BlacklistExemptionHandler
}

// New 创建 ops 域模块。
func New() *Module {
	dictSvc := opsservice.NewDictionaryService()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dictSvc.LoadCache(ctx); err != nil {
		panic(fmt.Errorf("初始化字典缓存失败: %w", err))
	}

	return &Module{
		Audit: handler.NewAuditHandlerWithDeps(handler.AuditHandlerDeps{
			Repo: repository.NewAuditLogRepository(database.DB),
		}),
		PlatformAudit: handler.NewPlatformAuditHandlerWithDeps(handler.PlatformAuditHandlerDeps{
			Repo: repository.NewPlatformAuditLogRepository(),
		}),
		PlatformSettings: handler.NewPlatformSettingsHandlerWithDeps(handler.PlatformSettingsHandlerDeps{
			Repo: repository.NewPlatformSettingsRepository(),
		}),
		Dictionary: handler.NewDictionaryHandlerWithDeps(handler.DictionaryHandlerDeps{
			Service: dictSvc,
		}),
		CommandBlacklist: handler.NewCommandBlacklistHandlerWithDeps(handler.CommandBlacklistHandlerDeps{
			Service: opsservice.NewCommandBlacklistService(),
		}),
		BlacklistExemption: handler.NewBlacklistExemptionHandlerWithDeps(handler.BlacklistExemptionHandlerDeps{
			Service:       opsservice.NewBlacklistExemptionService(),
			TaskRepo:      repository.NewExecutionRepository(),
			BlacklistRepo: repository.NewCommandBlacklistRepository(),
		}),
	}
}
