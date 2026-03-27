package ops

import "github.com/company/auto-healing/internal/handler"

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
	return &Module{
		Audit:              handler.NewAuditHandler(),
		PlatformAudit:      handler.NewPlatformAuditHandler(),
		PlatformSettings:   handler.NewPlatformSettingsHandler(),
		Dictionary:         handler.NewDictionaryHandler(),
		CommandBlacklist:   handler.NewCommandBlacklistHandler(),
		BlacklistExemption: handler.NewBlacklistExemptionHandler(),
	}
}
