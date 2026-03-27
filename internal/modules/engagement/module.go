package engagement

import (
	"github.com/company/auto-healing/internal/database"
	engagementhttp "github.com/company/auto-healing/internal/modules/engagement/httpapi"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/company/auto-healing/internal/notification"
	platformevents "github.com/company/auto-healing/internal/platform/events"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
	"github.com/company/auto-healing/internal/repository"
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
			NotificationRepo: repository.NewNotificationRepository(db),
		}),
		Dashboard: engagementhttp.NewDashboardHandlerWithDeps(engagementhttp.DashboardHandlerDeps{
			DashboardRepo: engagementrepo.NewDashboardRepository(),
			WorkspaceRepo: engagementrepo.NewWorkspaceRepository(),
			RoleRepo:      repository.NewRoleRepository(),
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
			SiteMessageRepo:      repository.NewSiteMessageRepository(),
			PlatformSettingsRepo: settingsrepo.NewPlatformSettingsRepository(),
			EventBus:             platformevents.GetMessageEventBus(),
			TenantRepo:           repository.NewTenantRepository(),
			UserRepo:             repository.NewUserRepository(),
		}),
		Workbench: engagementhttp.NewWorkbenchHandlerWithDeps(engagementhttp.WorkbenchHandlerDeps{
			WorkbenchRepo: engagementrepo.NewWorkbenchRepository(db),
			ActivityRepo:  engagementrepo.NewUserActivityRepository(),
			UserRepo:      repository.NewUserRepository(),
		}),
	}
}
