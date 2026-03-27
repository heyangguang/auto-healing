package healing

import (
	"github.com/company/auto-healing/internal/database"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	"gorm.io/gorm"
)

// NewDryRunExecutor 保留兼容零参构造，生产路径应使用显式 deps。
func NewDryRunExecutor() *DryRunExecutor {
	return NewDryRunExecutorWithDB(database.DB)
}

func NewDryRunExecutorWithDB(db *gorm.DB) *DryRunExecutor {
	return NewDryRunExecutorWithDeps(DryRunExecutorDeps{
		TaskRepo:         automationrepo.NewExecutionRepositoryWithDB(db),
		CMDBRepo:         cmdbrepo.NewCMDBItemRepositoryWithDB(db),
		NotificationRepo: engagementrepo.NewNotificationRepository(db),
	})
}
