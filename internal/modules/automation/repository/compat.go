package repository

import "github.com/company/auto-healing/internal/database"

func NewExecutionRepository() *ExecutionRepository {
	return NewExecutionRepositoryWithDB(database.DB)
}

func NewFlowInstanceRepository() *FlowInstanceRepository {
	return NewFlowInstanceRepositoryWithDB(database.DB)
}

func NewHealingFlowRepository() *HealingFlowRepository {
	return NewHealingFlowRepositoryWithDB(database.DB)
}

func NewApprovalTaskRepository() *ApprovalTaskRepository {
	return NewApprovalTaskRepositoryWithDB(database.DB)
}

func NewFlowLogRepository() *FlowLogRepository {
	return NewFlowLogRepositoryWithDB(database.DB)
}

func NewHealingRuleRepository() *HealingRuleRepository {
	return NewHealingRuleRepositoryWithDB(database.DB)
}

func NewScheduleRepository() *ScheduleRepository {
	return NewScheduleRepositoryWithDB(database.DB)
}
