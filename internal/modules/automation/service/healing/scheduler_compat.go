package healing

import (
	"github.com/company/auto-healing/internal/database"
	"gorm.io/gorm"
)

func DefaultSchedulerDeps(executor *FlowExecutor) SchedulerDeps {
	return DefaultSchedulerDepsWithDB(database.DB, executor)
}

func DefaultSchedulerRuntimeDeps() SchedulerDeps {
	return DefaultSchedulerRuntimeDepsWithDB(database.DB)
}

// NewScheduler 保留兼容零参构造，生产路径应使用显式 deps。
func NewScheduler() *Scheduler {
	return NewSchedulerWithDB(database.DB)
}

func NewSchedulerWithDB(db *gorm.DB) *Scheduler {
	return NewSchedulerWithDeps(DefaultSchedulerRuntimeDepsWithDB(db))
}
