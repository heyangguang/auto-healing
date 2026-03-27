package scheduler

import (
	gitService "github.com/company/auto-healing/internal/modules/integrations/service/git"
	pluginService "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
)

func newGitSchedulerForTest() *GitScheduler {
	return NewGitSchedulerWithDeps(GitSchedulerDeps{
		GitService: &gitService.Service{},
	})
}

func newPluginSchedulerForTest() *PluginScheduler {
	return NewPluginSchedulerWithDeps(PluginSchedulerDeps{
		PluginService: &pluginService.Service{},
		CMDBService:   &pluginService.CMDBService{},
	})
}
