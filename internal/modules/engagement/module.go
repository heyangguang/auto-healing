package engagement

import (
	"github.com/company/auto-healing/internal/database"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	engagementhttp "github.com/company/auto-healing/internal/modules/engagement/httpapi"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/company/auto-healing/internal/notification"
	platformevents "github.com/company/auto-healing/internal/platform/events"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
)

// Module 聚合 engagement 域处理器构造。
type Module struct {
	Notification *engagementhttp.NotificationHandler
	Dashboard    *engagementhttp.DashboardHandler
	Preference   *engagementhttp.PreferenceHandler
	Activity     *engagementhttp.UserActivityHandler
	Search       *engagementhttp.SearchHandler
	SiteMessage  *engagementhttp.SiteMessageHandler
	Workbench    *engagementhttp.WorkbenchHandler
}

// New 创建 engagement 域模块。
func New() *Module {
	db := database.DB
	return &Module{
		Notification: engagementhttp.NewNotificationHandlerWithDeps(engagementhttp.NotificationHandlerDeps{
			Service:          notification.NewConfiguredService(db),
			NotificationRepo: engagementrepo.NewNotificationRepository(db),
		}),
		Dashboard: engagementhttp.NewDashboardHandlerWithDeps(engagementhttp.DashboardHandlerDeps{
			DashboardRepo: engagementrepo.NewDashboardRepository(),
			WorkspaceRepo: engagementrepo.NewWorkspaceRepository(),
			RoleRepo:      accessrepo.NewRoleRepository(),
		}),
		Preference: engagementhttp.NewPreferenceHandlerWithDeps(engagementhttp.PreferenceHandlerDeps{
			PreferenceRepo: engagementrepo.NewUserPreferenceRepository(),
		}),
		Activity: engagementhttp.NewUserActivityHandlerWithDeps(engagementhttp.UserActivityHandlerDeps{
			Repo: engagementrepo.NewUserActivityRepository(),
		}),
		Search: engagementhttp.NewSearchHandlerWithDeps(engagementhttp.SearchHandlerDeps{
			Repo: engagementrepo.NewSearchRepository(),
		}),
		SiteMessage: engagementhttp.NewSiteMessageHandlerWithDeps(engagementhttp.SiteMessageHandlerDeps{
			SiteMessageRepo:      engagementrepo.NewSiteMessageRepository(),
			PlatformSettingsRepo: settingsrepo.NewPlatformSettingsRepository(),
			EventBus:             platformevents.GetMessageEventBus(),
			TenantRepo:           accessrepo.NewTenantRepository(),
			UserRepo:             accessrepo.NewUserRepository(),
		}),
		Workbench: engagementhttp.NewWorkbenchHandlerWithDeps(engagementhttp.WorkbenchHandlerDeps{
			WorkbenchRepo: engagementrepo.NewWorkbenchRepository(db),
			ActivityRepo:  engagementrepo.NewUserActivityRepository(),
			UserRepo:      accessrepo.NewUserRepository(),
		}),
	}
}
