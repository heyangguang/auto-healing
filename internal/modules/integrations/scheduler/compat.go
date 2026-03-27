package scheduler

import (
	"github.com/company/auto-healing/internal/database"
	gitService "github.com/company/auto-healing/internal/modules/integrations/service/git"
	pluginService "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
	"gorm.io/gorm"
	"time"
)

func DefaultGitSchedulerDeps() GitSchedulerDeps {
	return DefaultGitSchedulerDepsWithDB(database.DB)
}

func DefaultPluginSchedulerDeps() PluginSchedulerDeps {
	return DefaultPluginSchedulerDepsWithDB(database.DB)
}

func DefaultGitSchedulerDepsWithDB(db *gorm.DB) GitSchedulerDeps {
	return GitSchedulerDeps{
		GitService: gitService.NewServiceWithDB(db),
		DB:         db,
		Interval:   60 * time.Second,
		InFlight:   schedulerx.NewInFlightSet(),
		Now:        time.Now,
	}
}

func DefaultPluginSchedulerDepsWithDB(db *gorm.DB) PluginSchedulerDeps {
	return PluginSchedulerDeps{
		PluginService: pluginService.NewServiceWithDB(db),
		CMDBService:   pluginService.NewCMDBServiceWithDB(db),
		DB:            db,
		Interval:      30 * time.Second,
		InFlight:      schedulerx.NewInFlightSet(),
		Now:           time.Now,
	}
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
