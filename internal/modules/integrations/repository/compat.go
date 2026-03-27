package repository

import "github.com/company/auto-healing/internal/database"

func NewGitRepositoryRepository() *GitRepositoryRepository {
	return NewGitRepositoryRepositoryWithDB(database.DB)
}

func NewPlaybookRepository() *PlaybookRepository {
	return NewPlaybookRepositoryWithDB(database.DB)
}

func NewPluginRepository() *PluginRepository {
	return NewPluginRepositoryWithDB(database.DB)
}

func NewPluginSyncLogRepository() *PluginSyncLogRepository {
	return NewPluginSyncLogRepositoryWithDB(database.DB)
}
