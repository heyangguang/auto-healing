package repository

import automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"

type ExecutionRepository = automationrepo.ExecutionRepository
type TaskListOptions = automationrepo.TaskListOptions
type RunListOptions = automationrepo.RunListOptions

type FlowInstanceRepository = automationrepo.FlowInstanceRepository
type FlowInstanceListOptions = automationrepo.FlowInstanceListOptions
type FlowInstanceSummary = automationrepo.FlowInstanceSummary

type FlowLogRepository = automationrepo.FlowLogRepository

type HealingFlowRepository = automationrepo.HealingFlowRepository
type HealingRuleRepository = automationrepo.HealingRuleRepository
type ApprovalTaskRepository = automationrepo.ApprovalTaskRepository

type ScheduleRepository = automationrepo.ScheduleRepository
type ScheduleListOptions = automationrepo.ScheduleListOptions
type ScheduleTimelineItem = automationrepo.ScheduleTimelineItem

var (
	ErrHealingFlowNotFound       = automationrepo.ErrHealingFlowNotFound
	ErrHealingRuleNotFound       = automationrepo.ErrHealingRuleNotFound
	ErrFlowInstanceNotFound      = automationrepo.ErrFlowInstanceNotFound
	ErrApprovalTaskNotFound      = automationrepo.ErrApprovalTaskNotFound
	ErrApprovalTaskNotPending    = automationrepo.ErrApprovalTaskNotPending
	ErrFlowInstanceStateConflict = automationrepo.ErrFlowInstanceStateConflict
	ErrScheduleNotFound          = automationrepo.ErrScheduleNotFound
)

func NewExecutionRepository() *ExecutionRepository {
	return automationrepo.NewExecutionRepository()
}

func NewFlowInstanceRepository() *FlowInstanceRepository {
	return automationrepo.NewFlowInstanceRepository()
}

func NewFlowLogRepository() *FlowLogRepository {
	return automationrepo.NewFlowLogRepository()
}

func NewHealingFlowRepository() *HealingFlowRepository {
	return automationrepo.NewHealingFlowRepository()
}

func NewHealingRuleRepository() *HealingRuleRepository {
	return automationrepo.NewHealingRuleRepository()
}

func NewApprovalTaskRepository() *ApprovalTaskRepository {
	return automationrepo.NewApprovalTaskRepository()
}

func NewScheduleRepository() *ScheduleRepository {
	return automationrepo.NewScheduleRepository()
}
