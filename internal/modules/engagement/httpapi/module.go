package httpapi

import "github.com/company/auto-healing/internal/handler"

type Dependencies struct {
	Activity     *handler.UserActivityHandler
	Dashboard    *handler.DashboardHandler
	Notification *handler.NotificationHandler
	Preference   *handler.PreferenceHandler
	Search       *handler.SearchHandler
	SiteMessage  *handler.SiteMessageHandler
	Workbench    *handler.WorkbenchHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
