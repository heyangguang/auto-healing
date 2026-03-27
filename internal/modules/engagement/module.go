package engagement

import (
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	engagementhttp "github.com/company/auto-healing/internal/modules/engagement/httpapi"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	platformevents "github.com/company/auto-healing/internal/platform/events"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
	"gorm.io/gorm"
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

type ModuleDeps struct {
	NotificationService  *notification.Service
	NotificationRepo     *engagementrepo.NotificationRepository
	DashboardRepo        *engagementrepo.DashboardRepository
	WorkspaceRepo        *engagementrepo.WorkspaceRepository
	WorkbenchRepo        *engagementrepo.WorkbenchRepository
	RoleRepo             *accessrepo.RoleRepository
	PreferenceRepo       *engagementrepo.UserPreferenceRepository
	ActivityRepo         *engagementrepo.UserActivityRepository
	SearchRepo           *engagementrepo.SearchRepository
	SiteMessageRepo      *engagementrepo.SiteMessageRepository
	PlatformSettingsRepo *settingsrepo.PlatformSettingsRepository
	EventBus             *platformevents.MessageEventBus
	TenantRepo           *accessrepo.TenantRepository
	UserRepo             *accessrepo.UserRepository
}

func NewWithDB(db *gorm.DB) *Module {
	return NewWithDeps(DefaultModuleDepsWithDB(db))
}

func DefaultModuleDepsWithDB(db *gorm.DB) ModuleDeps {
	settingsRepo := settingsrepo.NewPlatformSettingsRepositoryWithDB(db)
	notificationRepo := engagementrepo.NewNotificationRepository(db)
	activityRepo := engagementrepo.NewUserActivityRepositoryWithDB(db)
	userRepo := accessrepo.NewUserRepositoryWithDB(db)
	return ModuleDeps{
		NotificationService: notification.NewConfiguredServiceWithDeps(notification.ConfiguredServiceDeps{
			Repo:            notificationRepo,
			HealingFlowRepo: automationrepo.NewHealingFlowRepositoryWithDB(db),
		}),
		NotificationRepo:     notificationRepo,
		DashboardRepo:        engagementrepo.NewDashboardRepositoryWithDB(db),
		WorkspaceRepo:        engagementrepo.NewWorkspaceRepositoryWithDB(db),
		WorkbenchRepo:        engagementrepo.NewWorkbenchRepository(db),
		RoleRepo:             accessrepo.NewRoleRepositoryWithDB(db),
		PreferenceRepo:       engagementrepo.NewUserPreferenceRepositoryWithDB(db),
		ActivityRepo:         activityRepo,
		SearchRepo:           engagementrepo.NewSearchRepositoryWithDB(db),
		SiteMessageRepo:      engagementrepo.NewSiteMessageRepositoryWithDB(db),
		PlatformSettingsRepo: settingsRepo,
		EventBus:             platformevents.NewMessageEventBus(),
		TenantRepo:           accessrepo.NewTenantRepositoryWithDB(db),
		UserRepo:             userRepo,
	}
}

func NewWithDeps(deps ModuleDeps) *Module {
	return &Module{
		Notification: engagementhttp.NewNotificationHandlerWithDeps(engagementhttp.NotificationHandlerDeps{
			Service:          deps.NotificationService,
			NotificationRepo: deps.NotificationRepo,
		}),
		Dashboard: engagementhttp.NewDashboardHandlerWithDeps(engagementhttp.DashboardHandlerDeps{
			DashboardRepo: deps.DashboardRepo,
			WorkspaceRepo: deps.WorkspaceRepo,
			RoleRepo:      deps.RoleRepo,
		}),
		Preference: engagementhttp.NewPreferenceHandlerWithDeps(engagementhttp.PreferenceHandlerDeps{
			PreferenceRepo: deps.PreferenceRepo,
		}),
		Activity: engagementhttp.NewUserActivityHandlerWithDeps(engagementhttp.UserActivityHandlerDeps{
			Repo: deps.ActivityRepo,
		}),
		Search: engagementhttp.NewSearchHandlerWithDeps(engagementhttp.SearchHandlerDeps{
			Repo: deps.SearchRepo,
		}),
		SiteMessage: engagementhttp.NewSiteMessageHandlerWithDeps(engagementhttp.SiteMessageHandlerDeps{
			SiteMessageRepo:      deps.SiteMessageRepo,
			PlatformSettingsRepo: deps.PlatformSettingsRepo,
			EventBus:             deps.EventBus,
			TenantRepo:           deps.TenantRepo,
			UserRepo:             deps.UserRepo,
		}),
		Workbench: engagementhttp.NewWorkbenchHandlerWithDeps(engagementhttp.WorkbenchHandlerDeps{
			WorkbenchRepo: deps.WorkbenchRepo,
			ActivityRepo:  deps.ActivityRepo,
			UserRepo:      deps.UserRepo,
		}),
	}
}
