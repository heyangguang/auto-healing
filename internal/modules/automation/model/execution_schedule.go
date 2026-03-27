package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	ScheduleTypeCron = "cron"
	ScheduleTypeOnce = "once"
)

const (
	ScheduleStatusRunning    = "running"
	ScheduleStatusPending    = "pending"
	ScheduleStatusCompleted  = "completed"
	ScheduleStatusDisabled   = "disabled"
	ScheduleStatusAutoPaused = "auto_paused"
)

// ExecutionSchedule 定时任务调度配置
type ExecutionSchedule struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID     *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	Name         string     `json:"name" gorm:"type:varchar(200);not null"`
	TaskID       uuid.UUID  `json:"task_id" gorm:"type:uuid;not null"`
	ScheduleType string     `json:"schedule_type" gorm:"type:varchar(10);default:'cron'"`
	ScheduleExpr *string    `json:"schedule_expr,omitempty" gorm:"type:varchar(50)"`
	ScheduledAt  *time.Time `json:"scheduled_at,omitempty"`
	Status       string     `json:"status" gorm:"type:varchar(20);default:'disabled'"`
	NextRunAt    *time.Time `json:"next_run_at,omitempty"`
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`
	Enabled      bool       `json:"enabled"`
	Description  string     `json:"description,omitempty" gorm:"type:text"`

	MaxFailures         int    `json:"max_failures" gorm:"default:5"`
	ConsecutiveFailures int    `json:"consecutive_failures" gorm:"default:0"`
	PauseReason         string `json:"pause_reason,omitempty" gorm:"type:varchar(500)"`

	TargetHostsOverride string      `json:"target_hosts_override,omitempty" gorm:"type:text"`
	ExtraVarsOverride   JSON        `json:"extra_vars_override,omitempty" gorm:"type:jsonb"`
	SecretsSourceIDs    StringArray `json:"secrets_source_ids,omitempty" gorm:"type:jsonb;default:'[]'"`
	SkipNotification    bool        `json:"skip_notification" gorm:"default:false"`

	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt time.Time `json:"updated_at" gorm:"default:now()"`

	Task *ExecutionTask `json:"task,omitempty" gorm:"foreignKey:TaskID"`
}

func (ExecutionSchedule) TableName() string {
	return "execution_schedules"
}

func (s *ExecutionSchedule) IsCron() bool {
	return s.ScheduleType == ScheduleTypeCron
}

func (s *ExecutionSchedule) IsOnce() bool {
	return s.ScheduleType == ScheduleTypeOnce
}

func (s *ExecutionSchedule) CalculateStatus() string {
	if !s.Enabled {
		return ScheduleStatusDisabled
	}
	if s.IsCron() {
		return ScheduleStatusRunning
	}
	if s.LastRunAt != nil {
		return ScheduleStatusCompleted
	}
	return ScheduleStatusPending
}
