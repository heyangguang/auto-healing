package model

import (
	"time"

	"github.com/google/uuid"
)

// ExecutionRun 执行记录模型
type ExecutionRun struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	TaskID      uuid.UUID  `json:"task_id" gorm:"type:uuid;not null"`
	Status      string     `json:"status" gorm:"type:varchar(50);default:'pending'"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	Stats       JSON       `json:"stats,omitempty" gorm:"type:jsonb;default:'{}'"`
	Stdout      string     `json:"stdout,omitempty" gorm:"type:text"`
	Stderr      string     `json:"stderr,omitempty" gorm:"type:text"`
	TriggeredBy string     `json:"triggered_by,omitempty" gorm:"type:varchar(200)"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`

	RuntimeTargetHosts      string      `json:"runtime_target_hosts,omitempty" gorm:"type:text"`
	RuntimeSecretsSourceIDs StringArray `json:"runtime_secrets_source_ids,omitempty" gorm:"type:jsonb;default:'[]'"`
	RuntimeExtraVars        JSON        `json:"runtime_extra_vars,omitempty" gorm:"type:jsonb;default:'{}'"`
	RuntimeSkipNotification bool        `json:"runtime_skip_notification,omitempty" gorm:"default:false"`

	Task *ExecutionTask `json:"task,omitempty" gorm:"foreignKey:TaskID"`
	Logs []ExecutionLog `json:"logs,omitempty" gorm:"foreignKey:RunID"`
}

func (ExecutionRun) TableName() string {
	return "execution_runs"
}
