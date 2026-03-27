package model

import (
	"time"

	"github.com/google/uuid"
)

type ExecutionLog struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	RunID              uuid.UUID  `json:"run_id" gorm:"type:uuid;not null"`
	WorkflowInstanceID *uuid.UUID `json:"workflow_instance_id,omitempty" gorm:"type:uuid"`
	NodeExecutionID    *uuid.UUID `json:"node_execution_id,omitempty" gorm:"type:uuid"`
	LogLevel           string     `json:"log_level" gorm:"type:varchar(20);not null"`
	Stage              string     `json:"stage" gorm:"type:varchar(50);not null"`
	Message            string     `json:"message" gorm:"type:text;not null"`
	Host               string     `json:"host,omitempty" gorm:"type:varchar(200)"`
	TaskName           string     `json:"task_name,omitempty" gorm:"type:varchar(200)"`
	PlayName           string     `json:"play_name,omitempty" gorm:"type:varchar(200)"`
	Details            JSON       `json:"details" gorm:"type:jsonb;default:'{}'"`
	Sequence           int        `json:"sequence" gorm:"not null"`
	CreatedAt          time.Time  `json:"created_at" gorm:"default:now()"`

	Run *ExecutionRun `json:"run,omitempty" gorm:"foreignKey:RunID"`
}

func (ExecutionLog) TableName() string {
	return "execution_logs"
}
