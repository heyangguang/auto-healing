package model

import automodel "github.com/company/auto-healing/internal/modules/automation/model"

const (
	ScheduleTypeCron = automodel.ScheduleTypeCron
	ScheduleTypeOnce = automodel.ScheduleTypeOnce

	ScheduleStatusRunning    = automodel.ScheduleStatusRunning
	ScheduleStatusPending    = automodel.ScheduleStatusPending
	ScheduleStatusCompleted  = automodel.ScheduleStatusCompleted
	ScheduleStatusDisabled   = automodel.ScheduleStatusDisabled
	ScheduleStatusAutoPaused = automodel.ScheduleStatusAutoPaused

	FlowInstanceStatusPending         = automodel.FlowInstanceStatusPending
	FlowInstanceStatusRunning         = automodel.FlowInstanceStatusRunning
	FlowInstanceStatusWaitingApproval = automodel.FlowInstanceStatusWaitingApproval
	FlowInstanceStatusCompleted       = automodel.FlowInstanceStatusCompleted
	FlowInstanceStatusFailed          = automodel.FlowInstanceStatusFailed
	FlowInstanceStatusCancelled       = automodel.FlowInstanceStatusCancelled

	ApprovalTaskStatusPending   = automodel.ApprovalTaskStatusPending
	ApprovalTaskStatusApproved  = automodel.ApprovalTaskStatusApproved
	ApprovalTaskStatusRejected  = automodel.ApprovalTaskStatusRejected
	ApprovalTaskStatusExpired   = automodel.ApprovalTaskStatusExpired
	ApprovalTaskStatusCancelled = automodel.ApprovalTaskStatusCancelled

	NodeStatusPending         = automodel.NodeStatusPending
	NodeStatusRunning         = automodel.NodeStatusRunning
	NodeStatusSuccess         = automodel.NodeStatusSuccess
	NodeStatusPartial         = automodel.NodeStatusPartial
	NodeStatusFailed          = automodel.NodeStatusFailed
	NodeStatusSkipped         = automodel.NodeStatusSkipped
	NodeStatusWaitingApproval = automodel.NodeStatusWaitingApproval

	SSEEventFlowStart    = automodel.SSEEventFlowStart
	SSEEventNodeStart    = automodel.SSEEventNodeStart
	SSEEventNodeLog      = automodel.SSEEventNodeLog
	SSEEventNodeComplete = automodel.SSEEventNodeComplete
	SSEEventFlowComplete = automodel.SSEEventFlowComplete

	TriggerModeAuto   = automodel.TriggerModeAuto
	TriggerModeManual = automodel.TriggerModeManual
	MatchModeAll      = automodel.MatchModeAll
	MatchModeAny      = automodel.MatchModeAny

	NodeTypeStart         = automodel.NodeTypeStart
	NodeTypeEnd           = automodel.NodeTypeEnd
	NodeTypeHostExtractor = automodel.NodeTypeHostExtractor
	NodeTypeCMDBValidator = automodel.NodeTypeCMDBValidator
	NodeTypeApproval      = automodel.NodeTypeApproval
	NodeTypeExecution     = automodel.NodeTypeExecution
	NodeTypeNotification  = automodel.NodeTypeNotification
	NodeTypeCondition     = automodel.NodeTypeCondition
	NodeTypeSetVariable   = automodel.NodeTypeSetVariable
	NodeTypeCompute       = automodel.NodeTypeCompute

	OperatorEquals   = automodel.OperatorEquals
	OperatorContains = automodel.OperatorContains
	OperatorIn       = automodel.OperatorIn
	OperatorRegex    = automodel.OperatorRegex
	OperatorGt       = automodel.OperatorGt
	OperatorLt       = automodel.OperatorLt
	OperatorGte      = automodel.OperatorGte
	OperatorLte      = automodel.OperatorLte

	LogLevelDebug = automodel.LogLevelDebug
	LogLevelInfo  = automodel.LogLevelInfo
	LogLevelWarn  = automodel.LogLevelWarn
	LogLevelError = automodel.LogLevelError
)

type ExecutionTask = automodel.ExecutionTask
type ExecutionRun = automodel.ExecutionRun
type ExecutionSchedule = automodel.ExecutionSchedule

type HealingFlow = automodel.HealingFlow
type HealingRule = automodel.HealingRule
type FlowInstance = automodel.FlowInstance
type ApprovalTask = automodel.ApprovalTask
type FlowExecutionLog = automodel.FlowExecutionLog

type FlowNode = automodel.FlowNode
type FlowNodePosition = automodel.FlowNodePosition
type FlowEdge = automodel.FlowEdge
type RuleCondition = automodel.RuleCondition

type Workflow = automodel.Workflow
type WorkflowNode = automodel.WorkflowNode
type WorkflowEdge = automodel.WorkflowEdge
type WorkflowInstance = automodel.WorkflowInstance
type NodeExecution = automodel.NodeExecution

type ExecutionLog = automodel.ExecutionLog
type WorkflowLog = automodel.WorkflowLog
