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

// New 创建 ops 域模块。
func New() *Module {
	return NewWithDB(database.DB)
}

func NewWithDB(db *gorm.DB) *Module {
	return NewWithDeps(DefaultModuleDepsWithDB(db))
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dictSvc.LoadCache(ctx); err != nil {
		panic(fmt.Errorf("初始化字典缓存失败: %w", err))
	}
	return ModuleDeps{
		DictionaryService:     dictSvc,
		CommandBlacklistSvc:   commandBlacklistSvc,
		BlacklistExemptionSvc: blacklistExemptionSvc,
		AuditRepo:             auditrepo.NewAuditLogRepository(db),
		PlatformAuditRepo:     auditrepo.NewPlatformAuditLogRepository(),
		PlatformSettingsRepo:  settingsrepo.NewPlatformSettingsRepositoryWithDB(db),
		CommandBlacklistRepo:  commandBlacklistRepo,
		ExecutionRepo:         automationrepo.NewExecutionRepository(),
	}
}

func NewWithDeps(deps ModuleDeps) *Module {
	return &Module{
		Audit: opshttp.NewAuditHandlerWithDeps(opshttp.AuditHandlerDeps{
			Repo: deps.AuditRepo,
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
