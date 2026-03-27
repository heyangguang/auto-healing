package httpapi

import "github.com/company/auto-healing/internal/handler"

type Dependencies struct {
	Audit              *handler.AuditHandler
	BlacklistExemption *handler.BlacklistExemptionHandler
	CommandBlacklist   *handler.CommandBlacklistHandler
	Dictionary         *handler.DictionaryHandler
	PlatformAudit      *handler.PlatformAuditHandler
	PlatformSettings   *handler.PlatformSettingsHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
