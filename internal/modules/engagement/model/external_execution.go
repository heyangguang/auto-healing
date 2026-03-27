package model

import projection "github.com/company/auto-healing/internal/modules/engagement/projection"

type GitRepository = projection.GitRepository
type Playbook = projection.Playbook
type PlaybookScanLog = projection.PlaybookScanLog
type ExecutionTask = projection.ExecutionTask
type ExecutionRun = projection.ExecutionRun
type ExecutionSchedule = projection.ExecutionSchedule
type WorkflowInstance = projection.WorkflowInstance
type HealingFlow = projection.HealingFlow
type HealingRule = projection.HealingRule
type FlowInstance = projection.FlowInstance
type ApprovalTask = projection.ApprovalTask

const (
	ScheduleTypeCron = projection.ScheduleTypeCron
	ScheduleTypeOnce = projection.ScheduleTypeOnce
)
