package model

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog 租户级审计日志模型
type AuditLog struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	UserID         *uuid.UUID `json:"user_id,omitempty" gorm:"type:uuid"`
	Username       string     `json:"username,omitempty" gorm:"type:varchar(200)"`
	IPAddress      string     `json:"ip_address,omitempty" gorm:"type:varchar(45)"`
	UserAgent      string     `json:"user_agent,omitempty" gorm:"type:text"`
	Category       string     `json:"category" gorm:"type:varchar(20);not null;default:'operation'"` // login | operation
	Action         string     `json:"action" gorm:"type:varchar(100);not null"`                      // create, update, delete, execute, login, logout
	ResourceType   string     `json:"resource_type" gorm:"type:varchar(100);not null"`
	ResourceID     *uuid.UUID `json:"resource_id,omitempty" gorm:"type:uuid"`
	ResourceName   string     `json:"resource_name,omitempty" gorm:"type:varchar(200)"`
	RequestMethod  string     `json:"request_method,omitempty" gorm:"type:varchar(10)"`
	RequestPath    string     `json:"request_path,omitempty" gorm:"type:varchar(500)"`
	RequestBody    JSON       `json:"request_body,omitempty" gorm:"type:jsonb"`
	ResponseStatus *int       `json:"response_status,omitempty"`
	Changes        JSON       `json:"changes,omitempty" gorm:"type:jsonb"`
	Status         string     `json:"status" gorm:"type:varchar(20);not null"` // success, failed
	ErrorMessage   string     `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt      time.Time  `json:"created_at" gorm:"default:now()"`

	// 关联
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName 表名
func (AuditLog) TableName() string {
	return "audit_logs"
}

// PlatformAuditLog 平台级审计日志模型（无租户隔离）
type PlatformAuditLog struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID         *uuid.UUID `json:"user_id,omitempty" gorm:"type:uuid"`
	Username       string     `json:"username,omitempty" gorm:"type:varchar(200)"`
	IPAddress      string     `json:"ip_address,omitempty" gorm:"type:varchar(45)"`
	UserAgent      string     `json:"user_agent,omitempty" gorm:"type:text"`
	Category       string     `json:"category" gorm:"type:varchar(20);not null;default:'operation'"` // login | operation
	Action         string     `json:"action" gorm:"type:varchar(100);not null"`
	ResourceType   string     `json:"resource_type" gorm:"type:varchar(100);not null"`
	ResourceID     *uuid.UUID `json:"resource_id,omitempty" gorm:"type:uuid"`
	ResourceName   string     `json:"resource_name,omitempty" gorm:"type:varchar(200)"`
	RequestMethod  string     `json:"request_method,omitempty" gorm:"type:varchar(10)"`
	RequestPath    string     `json:"request_path,omitempty" gorm:"type:varchar(500)"`
	RequestBody    JSON       `json:"request_body,omitempty" gorm:"type:jsonb"`
	ResponseStatus *int       `json:"response_status,omitempty"`
	Changes        JSON       `json:"changes,omitempty" gorm:"type:jsonb"`
	Status         string     `json:"status" gorm:"type:varchar(20);not null"`
	ErrorMessage   string     `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt      time.Time  `json:"created_at" gorm:"default:now()"`
}

// TableName 表名
func (PlatformAuditLog) TableName() string {
	return "platform_audit_logs"
}

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
