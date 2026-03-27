package repository

import "github.com/company/auto-healing/internal/database"

func NewDashboardRepository() *DashboardRepository {
	return NewDashboardRepositoryWithDB(database.DB)
}

func NewSearchRepository() *SearchRepository {
	return NewSearchRepositoryWithDB(database.DB)
}

func NewWorkspaceRepository() *WorkspaceRepository {
	return NewWorkspaceRepositoryWithDB(database.DB)
}

func NewUserActivityRepository() *UserActivityRepository {
	return NewUserActivityRepositoryWithDB(database.DB)
}

func NewUserPreferenceRepository() *UserPreferenceRepository {
	return NewUserPreferenceRepositoryWithDB(database.DB)
}

func NewSiteMessageRepository() *SiteMessageRepository {
	return NewSiteMessageRepositoryWithDB(database.DB)
}
