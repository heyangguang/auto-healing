package healing

import (
	"github.com/company/auto-healing/internal/database"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	"gorm.io/gorm"
)

// NewNodeExecutors 保留兼容零参构造，生产路径应使用显式 deps。
func NewNodeExecutors() *NodeExecutors {
	return NewNodeExecutorsWithDB(database.DB)
}

func NewNodeExecutorsWithDB(db *gorm.DB) *NodeExecutors {
	return NewNodeExecutorsWithDeps(NodeExecutorsDeps{
		CMDBRepo:         cmdbrepo.NewCMDBItemRepositoryWithDB(db),
		NotificationRepo: engagementrepo.NewNotificationRepository(db),
		NotificationSvc:  notification.NewConfiguredService(db),
	})
}
