package httpapi

type Dependencies struct {
	Audit              *AuditHandler
	BlacklistExemption *BlacklistExemptionHandler
	CommandBlacklist   *CommandBlacklistHandler
	Dictionary         *DictionaryHandler
	PlatformAudit      *PlatformAuditHandler
	PlatformSettings   *PlatformSettingsHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
