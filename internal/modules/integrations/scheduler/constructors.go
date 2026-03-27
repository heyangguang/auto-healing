package scheduler

import (
	"time"

	gitService "github.com/company/auto-healing/internal/modules/integrations/service/git"
	pluginService "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
	"gorm.io/gorm"
)

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

func NewGitSchedulerWithDB(db *gorm.DB) *GitScheduler {
	return NewGitSchedulerWithDeps(DefaultGitSchedulerDepsWithDB(db))
}

func NewPluginSchedulerWithDB(db *gorm.DB) *PluginScheduler {
	return NewPluginSchedulerWithDeps(DefaultPluginSchedulerDepsWithDB(db))
}
