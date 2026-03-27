package model

import (
	"time"

	integrationsmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

// ExecutionTask 执行任务模板
type ExecutionTask struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	Name               string     `json:"name" gorm:"type:varchar(200)"`
	PlaybookID         uuid.UUID  `json:"playbook_id" gorm:"type:uuid;not null"`
	WorkflowInstanceID *uuid.UUID `json:"workflow_instance_id,omitempty" gorm:"-"`
	NodeExecutionID    *uuid.UUID `json:"node_execution_id,omitempty" gorm:"-"`
	TargetHosts        string     `json:"target_hosts" gorm:"type:text;not null"`
	ExtraVars          JSON       `json:"extra_vars" gorm:"type:jsonb;default:'{}'"`

	ExecutorType     string      `json:"executor_type" gorm:"type:varchar(20);default:'local'"`
	Description      string      `json:"description" gorm:"type:text"`
	SecretsSourceIDs StringArray `json:"secrets_source_ids" gorm:"type:jsonb;default:'[]'"`

	NotificationConfig *modeltypes.TaskNotificationConfig `json:"notification_config,omitempty" gorm:"type:jsonb"`

	PlaybookVariablesSnapshot JSONArray `json:"playbook_variables_snapshot" gorm:"type:jsonb;default:'[]'"`
	NeedsReview               bool      `json:"needs_review" gorm:"default:false"`
	ChangedVariables          JSONArray `json:"changed_variables" gorm:"type:jsonb;default:'[]'"`

	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt time.Time `json:"updated_at" gorm:"default:now()"`

	Playbook      *integrationsmodel.Playbook `json:"playbook,omitempty" gorm:"foreignKey:PlaybookID"`
	Runs          []ExecutionRun              `json:"runs,omitempty" gorm:"foreignKey:TaskID"`
	ScheduleCount int                         `json:"schedule_count" gorm:"-"`
}

func (ExecutionTask) TableName() string {
	return "execution_tasks"
}
