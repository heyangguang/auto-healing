package projection

import (
	automationmodel "github.com/company/auto-healing/internal/modules/automation/model"
	integrationsmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
)

type JSON = modeltypes.JSON
type JSONArray = modeltypes.JSONArray
type NotificationTriggerConfig = modeltypes.NotificationTriggerConfig
type TaskNotificationConfig = modeltypes.TaskNotificationConfig
type GitRepository = integrationsmodel.GitRepository
type Playbook = integrationsmodel.Playbook
type PlaybookScanLog = integrationsmodel.PlaybookScanLog
type ExecutionTask = automationmodel.ExecutionTask
type ExecutionRun = automationmodel.ExecutionRun
type ExecutionSchedule = automationmodel.ExecutionSchedule
type WorkflowInstance = automationmodel.WorkflowInstance
type HealingFlow = automationmodel.HealingFlow
type HealingRule = automationmodel.HealingRule
type FlowInstance = automationmodel.FlowInstance
type ApprovalTask = automationmodel.ApprovalTask

const (
	ScheduleTypeCron = automationmodel.ScheduleTypeCron
	ScheduleTypeOnce = automationmodel.ScheduleTypeOnce
)
