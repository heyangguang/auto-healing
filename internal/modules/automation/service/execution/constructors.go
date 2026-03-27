package execution

import (
	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	"gorm.io/gorm"
)

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo:             automationrepo.NewExecutionRepositoryWithDB(db),
		GitRepo:          integrationrepo.NewGitRepositoryRepositoryWithDB(db),
		SecretsRepo:      secretsrepo.NewSecretsSourceRepositoryWithDB(db),
		CMDBRepo:         cmdbrepo.NewCMDBItemRepositoryWithDB(db),
		HealingFlowRepo:  automationrepo.NewHealingFlowRepositoryWithDB(db),
		WorkspaceManager: ansible.NewWorkspaceManager(),
		LocalExecutor:    ansible.NewLocalExecutor(),
		DockerExecutor:   ansible.NewDockerExecutor(),
		NotificationSvc:  notification.NewConfiguredService(db),
		BlacklistSvc: opsservice.NewCommandBlacklistServiceWithDeps(opsservice.CommandBlacklistServiceDeps{
			Repo: opsrepo.NewCommandBlacklistRepositoryWithDB(db),
		}),
		ExemptionSvc: opsservice.NewBlacklistExemptionServiceWithDeps(opsservice.BlacklistExemptionServiceDeps{
			Repo: opsrepo.NewBlacklistExemptionRepository(db),
		}),
		Lifecycle: newAsyncLifecycle(maxExecutionWorkers),
	}
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}
