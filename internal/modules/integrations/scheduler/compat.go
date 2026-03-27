package scheduler

import (
	"github.com/company/auto-healing/internal/database"
	"gorm.io/gorm"
)

func DefaultGitSchedulerDeps() GitSchedulerDeps {
	return DefaultGitSchedulerDepsWithDB(database.DB)
}

func DefaultPluginSchedulerDeps() PluginSchedulerDeps {
	return DefaultPluginSchedulerDepsWithDB(database.DB)
}

// NewGitScheduler 保留兼容零参构造，生产路径应使用显式 deps。
func NewGitScheduler() *GitScheduler {
	return NewGitSchedulerWithDB(database.DB)
}

func NewGitSchedulerWithDB(db *gorm.DB) *GitScheduler {
	return NewGitSchedulerWithDeps(DefaultGitSchedulerDepsWithDB(db))
}

// NewPluginScheduler 保留兼容零参构造，生产路径应使用显式 deps。
func NewPluginScheduler() *PluginScheduler {
	return NewPluginSchedulerWithDB(database.DB)
}

func NewPluginSchedulerWithDB(db *gorm.DB) *PluginScheduler {
	return NewPluginSchedulerWithDeps(DefaultPluginSchedulerDepsWithDB(db))
}
