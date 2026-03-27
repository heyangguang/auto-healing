package healing

import (
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	notificationSvc "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"gorm.io/gorm"
)

func DefaultFlowExecutorDeps(executionSvc *execution.Service, notificationService *notificationSvc.Service) FlowExecutorDeps {
	return DefaultFlowExecutorDepsWithDB(database.DB, executionSvc, notificationService)
}

func DefaultFlowExecutorRuntimeDeps() FlowExecutorDeps {
	return DefaultFlowExecutorRuntimeDepsWithDB(database.DB)
}

// NewFlowExecutor 保留兼容零参构造，生产路径应使用显式 deps。
func NewFlowExecutor() *FlowExecutor {
	return NewFlowExecutorWithDB(database.DB)
}

func NewFlowExecutorWithDB(db *gorm.DB) *FlowExecutor {
	return NewFlowExecutorWithDeps(DefaultFlowExecutorRuntimeDepsWithDB(db))
}

func NewFlowExecutorWithDependencies(executionSvc *execution.Service, notificationService *notificationSvc.Service) *FlowExecutor {
	return NewFlowExecutorWithDeps(DefaultFlowExecutorDeps(executionSvc, notificationService))
}
