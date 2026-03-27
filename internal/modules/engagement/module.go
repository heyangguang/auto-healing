package engagement

import (
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/handler"
	"github.com/company/auto-healing/internal/notification"
	platformevents "github.com/company/auto-healing/internal/platform/events"
	"github.com/company/auto-healing/internal/repository"
)

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
	db := database.DB
	return &Module{
		Notification: handler.NewNotificationHandlerWithDeps(handler.NotificationHandlerDeps{
			Service:          notification.NewConfiguredService(db),
			NotificationRepo: repository.NewNotificationRepository(db),
		}),
		Dashboard: handler.NewDashboardHandlerWithDeps(handler.DashboardHandlerDeps{
			DashboardRepo: repository.NewDashboardRepository(),
			WorkspaceRepo: repository.NewWorkspaceRepository(),
			RoleRepo:      repository.NewRoleRepository(),
		}),
		Preference: handler.NewPreferenceHandlerWithDeps(handler.PreferenceHandlerDeps{
			PreferenceRepo: repository.NewUserPreferenceRepository(),
		}),
		Activity: handler.NewUserActivityHandlerWithDeps(handler.UserActivityHandlerDeps{
			Repo: repository.NewUserActivityRepository(),
		}),
		Search: handler.NewSearchHandlerWithDeps(handler.SearchHandlerDeps{
			Repo: repository.NewSearchRepository(),
		}),
		SiteMessage: handler.NewSiteMessageHandlerWithDeps(handler.SiteMessageHandlerDeps{
			SiteMessageRepo:      repository.NewSiteMessageRepository(),
			PlatformSettingsRepo: repository.NewPlatformSettingsRepository(),
			EventBus:             platformevents.GetMessageEventBus(),
			TenantRepo:           repository.NewTenantRepository(),
			UserRepo:             repository.NewUserRepository(),
		}),
		Workbench: handler.NewWorkbenchHandlerWithDeps(handler.WorkbenchHandlerDeps{
			WorkbenchRepo: repository.NewWorkbenchRepository(db),
			ActivityRepo:  repository.NewUserActivityRepository(),
			UserRepo:      repository.NewUserRepository(),
		}),
	}
}
