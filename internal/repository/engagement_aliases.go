package repository

import "github.com/company/auto-healing/internal/database"
import engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
import "gorm.io/gorm"

type DashboardRepository = engagementrepo.DashboardRepository
type NotificationRepository = engagementrepo.NotificationRepository
type SearchRepository = engagementrepo.SearchRepository
type SiteMessageRepository = engagementrepo.SiteMessageRepository
type TemplateListOptions = engagementrepo.TemplateListOptions
type NotificationLogListOptions = engagementrepo.NotificationLogListOptions
type UserPreferenceRepository = engagementrepo.UserPreferenceRepository
type UserActivityRepository = engagementrepo.UserActivityRepository
type WorkbenchRepository = engagementrepo.WorkbenchRepository
type WorkspaceRepository = engagementrepo.WorkspaceRepository

func NewDashboardRepository() *DashboardRepository {
	return engagementrepo.NewDashboardRepository()
}

func NewDashboardRepositoryWithDB(db *gorm.DB) *DashboardRepository {
	return engagementrepo.NewDashboardRepositoryWithDB(db)
}

func NewSearchRepository() *SearchRepository {
	return engagementrepo.NewSearchRepository()
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return engagementrepo.NewNotificationRepository(db)
}

func NewSiteMessageRepository() *SiteMessageRepository {
	return engagementrepo.NewSiteMessageRepository()
}

func NewUserPreferenceRepository() *UserPreferenceRepository {
	return engagementrepo.NewUserPreferenceRepository()
}

func NewUserActivityRepository() *UserActivityRepository {
	return engagementrepo.NewUserActivityRepository()
}

func NewWorkbenchRepository() *WorkbenchRepository {
	return engagementrepo.NewWorkbenchRepository(database.DB)
}

func NewWorkspaceRepository() *WorkspaceRepository {
	return engagementrepo.NewWorkspaceRepository()
}

func NewWorkspaceRepositoryWithDB(db *gorm.DB) *WorkspaceRepository {
	return engagementrepo.NewWorkspaceRepositoryWithDB(db)
}
