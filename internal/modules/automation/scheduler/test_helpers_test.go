package scheduler

import (
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	executionService "github.com/company/auto-healing/internal/modules/automation/service/execution"
	scheduleService "github.com/company/auto-healing/internal/modules/automation/service/schedule"
)

func newExecutionSchedulerForTest() *ExecutionScheduler {
	return NewExecutionSchedulerWithDeps(ExecutionSchedulerDeps{
		ExecutionService: &executionService.Service{},
		ScheduleService:  &scheduleService.Service{},
		ScheduleRepo:     &automationrepo.ScheduleRepository{},
	})
}
