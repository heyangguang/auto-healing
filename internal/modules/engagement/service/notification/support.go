package notification

import "github.com/company/auto-healing/internal/config"

func NewConfiguredServiceWithDeps(deps ConfiguredServiceDeps) *Service {
	requireConfiguredServiceDeps(deps)
	runtime := loadConfiguredServiceRuntime()
	return newServiceWithRuntime(deps, runtime.systemName, runtime.systemURL, runtime.systemVersion)
}

type configuredServiceRuntime struct {
	systemName    string
	systemURL     string
	systemVersion string
}

func loadConfiguredServiceRuntime() configuredServiceRuntime {
	appCfg := config.GetAppConfig()
	return configuredServiceRuntime{
		systemName:    appCfg.Name,
		systemURL:     appCfg.URL,
		systemVersion: appCfg.Version,
	}
}
