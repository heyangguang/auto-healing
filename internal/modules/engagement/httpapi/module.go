package httpapi

type Dependencies struct {
	Activity     *UserActivityHandler
	Dashboard    *DashboardHandler
	Notification *NotificationHandler
	Preference   *PreferenceHandler
	Search       *SearchHandler
	SiteMessage  *SiteMessageHandler
	Workbench    *WorkbenchHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
