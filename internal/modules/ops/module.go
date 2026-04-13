package ops

import (
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	opshttp "github.com/company/auto-healing/internal/modules/ops/httpapi"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
	"gorm.io/gorm"
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

type ModuleDeps struct {
	DictionaryService     *opsservice.DictionaryService
	CommandBlacklistSvc   *opsservice.CommandBlacklistService
	BlacklistExemptionSvc *opsservice.BlacklistExemptionService
	AuditRepo             *auditrepo.AuditLogRepository
	PlatformAuditRepo     *auditrepo.PlatformAuditLogRepository
	PlatformSettingsRepo  *settingsrepo.PlatformSettingsRepository
	CommandBlacklistRepo  *opsrepo.CommandBlacklistRepository
	ExecutionRepo         *automationrepo.ExecutionRepository
}

func NewWithDB(db *gorm.DB) *Module {
	deps := DefaultModuleDepsWithDB(db)
	mustInitializeRuntimeWithDeps(deps)
	return NewWithDeps(deps)
}

func DefaultModuleDepsWithDB(db *gorm.DB) ModuleDeps {
	dictRepo := opsrepo.NewDictionaryRepositoryWithDB(db)
	dictSvc := opsservice.NewDictionaryServiceWithDeps(opsservice.DictionaryServiceDeps{Repo: dictRepo})
	commandBlacklistRepo := opsrepo.NewCommandBlacklistRepositoryWithDB(db)
	commandBlacklistSvc := opsservice.NewCommandBlacklistServiceWithDeps(opsservice.CommandBlacklistServiceDeps{
		Repo: commandBlacklistRepo,
	})
	blacklistExemptionSvc := opsservice.NewBlacklistExemptionServiceWithDeps(opsservice.BlacklistExemptionServiceDeps{
		Repo: opsrepo.NewBlacklistExemptionRepository(db),
	})
	return ModuleDeps{
		DictionaryService:     dictSvc,
		CommandBlacklistSvc:   commandBlacklistSvc,
		BlacklistExemptionSvc: blacklistExemptionSvc,
		AuditRepo:             auditrepo.NewAuditLogRepository(db),
		PlatformAuditRepo:     auditrepo.NewPlatformAuditLogRepositoryWithDB(db),
		PlatformSettingsRepo:  settingsrepo.NewPlatformSettingsRepositoryWithDB(db),
		CommandBlacklistRepo:  commandBlacklistRepo,
		ExecutionRepo:         automationrepo.NewExecutionRepositoryWithDB(db),
	}
}

func NewWithDeps(deps ModuleDeps) *Module {
	return &Module{
		Audit: opshttp.NewAuditHandlerWithDeps(opshttp.AuditHandlerDeps{
			Repo:         deps.AuditRepo,
			PlatformRepo: deps.PlatformAuditRepo,
		}),
		PlatformAudit: opshttp.NewPlatformAuditHandlerWithDeps(opshttp.PlatformAuditHandlerDeps{
			Repo: deps.PlatformAuditRepo,
		}),
		PlatformSettings: opshttp.NewPlatformSettingsHandlerWithDeps(opshttp.PlatformSettingsHandlerDeps{
			Repo: deps.PlatformSettingsRepo,
		}),
		Dictionary: opshttp.NewDictionaryHandlerWithDeps(opshttp.DictionaryHandlerDeps{
			Service: deps.DictionaryService,
		}),
		CommandBlacklist: opshttp.NewCommandBlacklistHandlerWithDeps(opshttp.CommandBlacklistHandlerDeps{
			Service: deps.CommandBlacklistSvc,
		}),
		BlacklistExemption: opshttp.NewBlacklistExemptionHandlerWithDeps(opshttp.BlacklistExemptionHandlerDeps{
			Service:       deps.BlacklistExemptionSvc,
			TaskRepo:      deps.ExecutionRepo,
			BlacklistRepo: deps.CommandBlacklistRepo,
		}),
	}
}
