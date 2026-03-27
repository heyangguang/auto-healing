package engagement

import "github.com/company/auto-healing/internal/handler"

// Module 聚合 engagement 域处理器构造。
type Module struct {
	Notification *handler.NotificationHandler
	Dashboard    *handler.DashboardHandler
	Preference   *handler.PreferenceHandler
	Activity     *handler.UserActivityHandler
	Search       *handler.SearchHandler
	SiteMessage  *handler.SiteMessageHandler
	Workbench    *handler.WorkbenchHandler
}

// New 创建 engagement 域模块。
func New() *Module {
	return &Module{
		Notification: handler.NewNotificationHandler(),
		Dashboard:    handler.NewDashboardHandler(),
		Preference:   handler.NewPreferenceHandler(),
		Activity:     handler.NewUserActivityHandler(),
		Search:       handler.NewSearchHandler(),
		SiteMessage:  handler.NewSiteMessageHandler(),
		Workbench:    handler.NewWorkbenchHandler(),
	}
}
