package model

import (
	"time"

	"github.com/google/uuid"
)

// ExecutionRun 执行记录模型
// 每次执行任务都会创建一条记录，用于保存执行状态和结果
type ExecutionRun struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TaskID      uuid.UUID  `json:"task_id" gorm:"type:uuid;not null"`
	Status      string     `json:"status" gorm:"type:varchar(50);default:'pending'"` // pending, running, success, failed, cancelled, timeout
	ExitCode    *int       `json:"exit_code,omitempty"`
	Stats       JSON       `json:"stats,omitempty" gorm:"type:jsonb;default:'{}'"`
	Stdout      string     `json:"stdout,omitempty" gorm:"type:text"` // 标准输出
	Stderr      string     `json:"stderr,omitempty" gorm:"type:text"` // 错误输出
	TriggeredBy string     `json:"triggered_by,omitempty" gorm:"type:varchar(200)"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`

	// 运行时参数快照（记录这次执行实际使用的参数，方便排错和重试）
	RuntimeTargetHosts      string      `json:"runtime_target_hosts,omitempty" gorm:"type:text"`
	RuntimeSecretsSourceIDs StringArray `json:"runtime_secrets_source_ids,omitempty" gorm:"type:jsonb;default:'[]'"`
	RuntimeExtraVars        JSON        `json:"runtime_extra_vars,omitempty" gorm:"type:jsonb;default:'{}'"`
	RuntimeSkipNotification bool        `json:"runtime_skip_notification,omitempty" gorm:"default:false"`

	// 关联
	Task *ExecutionTask `json:"task,omitempty" gorm:"foreignKey:TaskID"`
	Logs []ExecutionLog `json:"logs,omitempty" gorm:"foreignKey:RunID"`
}

// TableName 表名
func (ExecutionRun) TableName() string {
	return "execution_runs"
}
