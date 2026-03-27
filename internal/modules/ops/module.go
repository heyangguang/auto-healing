package ops

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	opshttp "github.com/company/auto-healing/internal/modules/ops/httpapi"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
)

// Module 聚合 ops 域处理器构造。
type Module struct {
	Audit              *opshttp.AuditHandler
	PlatformAudit      *opshttp.PlatformAuditHandler
	PlatformSettings   *opshttp.PlatformSettingsHandler
	Dictionary         *opshttp.DictionaryHandler
	CommandBlacklist   *opshttp.CommandBlacklistHandler
	BlacklistExemption *opshttp.BlacklistExemptionHandler
}

// New 创建 ops 域模块。
func New() *Module {
	dictRepo := opsrepo.NewDictionaryRepository()
	dictSvc := opsservice.NewDictionaryServiceWithDeps(opsservice.DictionaryServiceDeps{
		Repo: dictRepo,
	})
	commandBlacklistRepo := opsrepo.NewCommandBlacklistRepository()
	commandBlacklistSvc := opsservice.NewCommandBlacklistServiceWithDeps(opsservice.CommandBlacklistServiceDeps{
		Repo: commandBlacklistRepo,
	})
	blacklistExemptionSvc := opsservice.NewBlacklistExemptionServiceWithDeps(opsservice.BlacklistExemptionServiceDeps{
		Repo: opsrepo.NewBlacklistExemptionRepository(database.DB),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dictSvc.LoadCache(ctx); err != nil {
		panic(fmt.Errorf("初始化字典缓存失败: %w", err))
	}

	return &Module{
		Audit: opshttp.NewAuditHandlerWithDeps(opshttp.AuditHandlerDeps{
			Repo: auditrepo.NewAuditLogRepository(database.DB),
		}),
		PlatformAudit: opshttp.NewPlatformAuditHandlerWithDeps(opshttp.PlatformAuditHandlerDeps{
			Repo: auditrepo.NewPlatformAuditLogRepository(),
		}),
		PlatformSettings: opshttp.NewPlatformSettingsHandlerWithDeps(opshttp.PlatformSettingsHandlerDeps{
			Repo: settingsrepo.NewPlatformSettingsRepository(),
		}),
		Dictionary: opshttp.NewDictionaryHandlerWithDeps(opshttp.DictionaryHandlerDeps{
			Service: dictSvc,
		}),
		CommandBlacklist: opshttp.NewCommandBlacklistHandlerWithDeps(opshttp.CommandBlacklistHandlerDeps{
			Service: commandBlacklistSvc,
		}),
		BlacklistExemption: opshttp.NewBlacklistExemptionHandlerWithDeps(opshttp.BlacklistExemptionHandlerDeps{
			Service:       blacklistExemptionSvc,
			TaskRepo:      automationrepo.NewExecutionRepository(),
			BlacklistRepo: commandBlacklistRepo,
		}),
	}
}
