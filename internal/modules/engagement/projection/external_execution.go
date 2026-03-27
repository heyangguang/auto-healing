package projection

import (
	"time"

	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

type JSON = modeltypes.JSON
type JSONArray = modeltypes.JSONArray

type NotificationTriggerConfig struct {
	Enabled    bool        `json:"enabled"`
	ChannelIDs []uuid.UUID `json:"channel_ids,omitempty"`
	TemplateID *uuid.UUID  `json:"template_id,omitempty"`
}

type TaskNotificationConfig struct {
	Enabled   bool                       `json:"enabled"`
	OnStart   *NotificationTriggerConfig `json:"on_start,omitempty"`
	OnSuccess *NotificationTriggerConfig `json:"on_success,omitempty"`
	OnFailure *NotificationTriggerConfig `json:"on_failure,omitempty"`
}

func (c *TaskNotificationConfig) GetTriggerConfig(status string) *NotificationTriggerConfig {
	if c == nil || !c.Enabled {
		return nil
	}
	switch status {
	case "start":
		return c.OnStart
	case "success":
		return c.OnSuccess
	case "failed", "timeout":
		return c.OnFailure
	default:
		return nil
	}
}

type GitRepository struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	DefaultBranch string     `json:"default_branch"`
	Status        string     `json:"status"`
	LastSyncAt    *time.Time `json:"last_sync_at,omitempty"`
}

func (GitRepository) TableName() string {
	return "git_repositories"
}

type Playbook struct {
	ID           uuid.UUID      `json:"id"`
	RepositoryID uuid.UUID      `json:"repository_id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	FilePath     string         `json:"file_path"`
	Status       string         `json:"status"`
	Repository   *GitRepository `json:"repository,omitempty" gorm:"foreignKey:RepositoryID"`
}

func (Playbook) TableName() string {
	return "playbooks"
}

type PlaybookScanLog struct {
	ID          uuid.UUID `json:"id"`
	PlaybookID  uuid.UUID `json:"playbook_id"`
	TriggerType string    `json:"trigger_type"`
	CreatedAt   time.Time `json:"created_at"`
	Playbook    *Playbook `json:"playbook,omitempty" gorm:"foreignKey:PlaybookID"`
}

func (PlaybookScanLog) TableName() string {
	return "playbook_scan_logs"
}

type ExecutionTask struct {
	ID                 uuid.UUID               `json:"id"`
	PlaybookID         uuid.UUID               `json:"playbook_id"`
	Name               string                  `json:"name"`
	Description        string                  `json:"description"`
	TargetHosts        string                  `json:"target_hosts"`
	ExecutorType       string                  `json:"executor_type"`
	NotificationConfig *TaskNotificationConfig `json:"notification_config,omitempty" gorm:"type:jsonb"`
	Playbook           *Playbook               `json:"playbook,omitempty" gorm:"foreignKey:PlaybookID"`
}

func (ExecutionTask) TableName() string {
	return "execution_tasks"
}

type ExecutionRun struct {
	ID          uuid.UUID      `json:"id"`
	TenantID    *uuid.UUID     `json:"tenant_id,omitempty"`
	TaskID      uuid.UUID      `json:"task_id"`
	Status      string         `json:"status"`
	ExitCode    *int           `json:"exit_code,omitempty"`
	Stats       JSON           `json:"stats,omitempty" gorm:"type:jsonb;default:'{}'"`
	Stdout      string         `json:"stdout,omitempty"`
	Stderr      string         `json:"stderr,omitempty"`
	TriggeredBy string         `json:"triggered_by,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	Task        *ExecutionTask `json:"task,omitempty" gorm:"foreignKey:TaskID"`
}

func (ExecutionRun) TableName() string {
	return "execution_runs"
}

type ExecutionSchedule struct {
	ID           uuid.UUID      `json:"id"`
	TaskID       uuid.UUID      `json:"task_id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Enabled      bool           `json:"enabled"`
	ScheduleType string         `json:"schedule_type"`
	ScheduleExpr *string        `json:"schedule_expr,omitempty"`
	ScheduledAt  *time.Time     `json:"scheduled_at,omitempty"`
	Status       string         `json:"status"`
	NextRunAt    *time.Time     `json:"next_run_at,omitempty"`
	LastRunAt    *time.Time     `json:"last_run_at,omitempty"`
	Task         *ExecutionTask `json:"task,omitempty" gorm:"foreignKey:TaskID"`
}

func (ExecutionSchedule) TableName() string {
	return "execution_schedules"
}

const (
	ScheduleTypeCron = "cron"
	ScheduleTypeOnce = "once"
)

type WorkflowInstance struct {
	ID uuid.UUID `json:"id"`
}

func (WorkflowInstance) TableName() string {
	return "workflow_instances"
}

type HealingFlow struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
}

func (HealingFlow) TableName() string {
	return "healing_flows"
}

type HealingRule struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
}

func (HealingRule) TableName() string {
	return "healing_rules"
}

type FlowInstance struct {
	ID        uuid.UUID `json:"id"`
	FlowName  string    `json:"flow_name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (FlowInstance) TableName() string {
	return "flow_instances"
}

type ApprovalTask struct {
	ID             uuid.UUID `json:"id"`
	FlowInstanceID uuid.UUID `json:"flow_instance_id"`
	NodeID         string    `json:"node_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func (ApprovalTask) TableName() string {
	return "approval_tasks"
}
