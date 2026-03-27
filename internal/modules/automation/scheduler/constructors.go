package scheduler

import (
	"time"

	"github.com/company/auto-healing/internal/modules/automation/repository"
	executionService "github.com/company/auto-healing/internal/modules/automation/service/execution"
	scheduleService "github.com/company/auto-healing/internal/modules/automation/service/schedule"
	platformsched "github.com/company/auto-healing/internal/platform/schedulerx"
	"gorm.io/gorm"
)

func DefaultExecutionSchedulerDepsWithDB(db *gorm.DB) ExecutionSchedulerDeps {
	return ExecutionSchedulerDeps{
		ExecutionService: executionService.NewServiceWithDB(db),
		ScheduleService:  scheduleService.NewServiceWithDB(db),
		ScheduleRepo:     repository.NewScheduleRepositoryWithDB(db),
		DB:               db,
		Interval:         30 * time.Second,
		InFlight:         platformsched.NewInFlightSet(),
		Sem:              make(chan struct{}, 8),
	}
}

func NewExecutionSchedulerWithDB(db *gorm.DB) *ExecutionScheduler {
	return NewExecutionSchedulerWithDeps(DefaultExecutionSchedulerDepsWithDB(db))
}
