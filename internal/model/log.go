package model

import (
	"time"

	"github.com/google/uuid"
)

// ExecutionLog 执行日志模型
// 每条日志关联到一次执行记录 (ExecutionRun)
type ExecutionLog struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	RunID              uuid.UUID  `json:"run_id" gorm:"type:uuid;not null"` // 关联执行记录，通过 Run 可以找到 Task
	WorkflowInstanceID *uuid.UUID `json:"workflow_instance_id,omitempty" gorm:"type:uuid"`
	NodeExecutionID    *uuid.UUID `json:"node_execution_id,omitempty" gorm:"type:uuid"`
	LogLevel           string     `json:"log_level" gorm:"type:varchar(20);not null"` // debug, info, warn, error
	Stage              string     `json:"stage" gorm:"type:varchar(50);not null"`     // prepare, execute, cleanup
	Message            string     `json:"message" gorm:"type:text;not null"`
	Host               string     `json:"host,omitempty" gorm:"type:varchar(200)"`
	TaskName           string     `json:"task_name,omitempty" gorm:"type:varchar(200)"`
	PlayName           string     `json:"play_name,omitempty" gorm:"type:varchar(200)"`
	Details            JSON       `json:"details" gorm:"type:jsonb;default:'{}'"`
	Sequence           int        `json:"sequence" gorm:"not null"`
	CreatedAt          time.Time  `json:"created_at" gorm:"default:now()"`

	// 关联
	Run *ExecutionRun `json:"run,omitempty" gorm:"foreignKey:RunID"`
}

// TableName 表名
func (ExecutionLog) TableName() string {
	return "execution_logs"
}

// WorkflowLog 工作流执行日志模型
type WorkflowLog struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	WorkflowInstanceID uuid.UUID  `json:"workflow_instance_id" gorm:"type:uuid;not null"`
	NodeID             *uuid.UUID `json:"node_id,omitempty" gorm:"type:uuid"`
	LogLevel           string     `json:"log_level" gorm:"type:varchar(20);not null"`
	Stage              string     `json:"stage" gorm:"type:varchar(50);not null"`
	Message            string     `json:"message" gorm:"type:text;not null"`
	Details            JSON       `json:"details" gorm:"type:jsonb;default:'{}'"`
	Sequence           int        `json:"sequence" gorm:"not null"`
	CreatedAt          time.Time  `json:"created_at" gorm:"default:now()"`

	// 关联
	WorkflowInstance *WorkflowInstance `json:"workflow_instance,omitempty" gorm:"foreignKey:WorkflowInstanceID"`
	Node             *WorkflowNode     `json:"node,omitempty" gorm:"foreignKey:NodeID"`
}

// TableName 表名
func (WorkflowLog) TableName() string {
	return "workflow_logs"
}
