package scheduler

import (
	"github.com/company/auto-healing/internal/database"
	"gorm.io/gorm"
)

func DefaultExecutionSchedulerDeps() ExecutionSchedulerDeps {
	return DefaultExecutionSchedulerDepsWithDB(database.DB)
}

// NewExecutionScheduler 保留兼容零参构造，生产路径应使用显式 deps。
func NewExecutionScheduler() *ExecutionScheduler {
	return NewExecutionSchedulerWithDB(database.DB)
}

func NewExecutionSchedulerWithDB(db *gorm.DB) *ExecutionScheduler {
	return NewExecutionSchedulerWithDeps(DefaultExecutionSchedulerDepsWithDB(db))
}
