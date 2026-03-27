package httpapi

import (
	"github.com/company/auto-healing/internal/database"
	gitSvc "github.com/company/auto-healing/internal/modules/integrations/service/git"
	"github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	"github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	"gorm.io/gorm"
)

// NewGitRepoHandler 保留兼容零参构造，生产路径应使用显式 deps。
func NewGitRepoHandler() *GitRepoHandler {
	return NewGitRepoHandlerWithDeps(GitRepoHandlerDeps{
		Service: gitSvc.NewService(),
	})
}

// NewPlaybookHandler 保留兼容零参构造，生产路径应使用显式 deps。
func NewPlaybookHandler() *PlaybookHandler {
	return NewPlaybookHandlerWithDeps(PlaybookHandlerDeps{
		Service: playbook.NewService(),
	})
}

// NewPluginHandler 保留兼容零参构造，生产路径应使用显式 deps。
func NewPluginHandler() *PluginHandler {
	return NewPluginHandlerWithDB(database.DB)
}

func NewPluginHandlerWithDB(db *gorm.DB) *PluginHandler {
	return NewPluginHandlerWithDeps(PluginHandlerDeps{
		PluginService:   plugin.NewServiceWithDB(db),
		IncidentService: plugin.NewIncidentServiceWithDB(db),
	})
}

// NewCMDBHandler 保留兼容零参构造，生产路径应使用显式 deps。
func NewCMDBHandler() *CMDBHandler {
	return NewCMDBHandlerWithDB(database.DB)
}

func NewCMDBHandlerWithDB(db *gorm.DB) *CMDBHandler {
	return NewCMDBHandlerWithDeps(CMDBHandlerDeps{
		Service:       plugin.NewCMDBServiceWithDB(db),
		SecretService: secretsSvc.NewServiceWithDB(db),
	})
}
