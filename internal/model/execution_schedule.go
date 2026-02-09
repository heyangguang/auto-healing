package model

import (
	"time"

	"github.com/google/uuid"
)

// 调度类型常量
const (
	ScheduleTypeCron = "cron" // 循环调度：使用 Cron 表达式
	ScheduleTypeOnce = "once" // 单次调度：使用具体执行时间
)

// 调度状态常量
const (
	ScheduleStatusRunning   = "running"   // 运行中：循环调度 + 启用
	ScheduleStatusPending   = "pending"   // 待执行：单次调度 + 启用 + 未执行
	ScheduleStatusCompleted = "completed" // 已完成：单次调度 + 已执行
	ScheduleStatusDisabled  = "disabled"  // 已禁用：开关关闭
)

// ExecutionSchedule 定时任务调度配置
// 支持两种调度模式：cron（循环）和 once（单次）
type ExecutionSchedule struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name         string     `json:"name" gorm:"type:varchar(200);not null"`               // 调度名称
	TaskID       uuid.UUID  `json:"task_id" gorm:"type:uuid;not null"`                    // 关联的任务模板
	ScheduleType string     `json:"schedule_type" gorm:"type:varchar(10);default:'cron'"` // 调度类型：cron/once
	ScheduleExpr *string    `json:"schedule_expr,omitempty" gorm:"type:varchar(50)"`      // Cron 表达式（仅 cron 模式）
	ScheduledAt  *time.Time `json:"scheduled_at,omitempty"`                               // 执行时间点（仅 once 模式）
	Status       string     `json:"status" gorm:"type:varchar(20);default:'disabled'"`    // 状态
	NextRunAt    *time.Time `json:"next_run_at,omitempty"`                                // 下次执行时间
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`                                // 上次执行时间
	Enabled      bool       `json:"enabled"`                                              // 是否启用
	Description  string     `json:"description,omitempty" gorm:"type:text"`               // 描述

	// 执行参数覆盖（可选，与手动执行保持一致）
	TargetHostsOverride string      `json:"target_hosts_override,omitempty" gorm:"type:text"`            // 覆盖目标主机
	ExtraVarsOverride   JSON        `json:"extra_vars_override,omitempty" gorm:"type:jsonb"`             // 覆盖变量
	SecretsSourceIDs    StringArray `json:"secrets_source_ids,omitempty" gorm:"type:jsonb;default:'[]'"` // 覆盖密钥源
	SkipNotification    bool        `json:"skip_notification" gorm:"default:false"`                      // 跳过通知

	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt time.Time `json:"updated_at" gorm:"default:now()"`

	// 关联
	Task *ExecutionTask `json:"task,omitempty" gorm:"foreignKey:TaskID"`
}

// TableName 表名
func (ExecutionSchedule) TableName() string {
	return "execution_schedules"
}

// IsCron 是否为循环调度
func (s *ExecutionSchedule) IsCron() bool {
	return s.ScheduleType == ScheduleTypeCron
}

// IsOnce 是否为单次调度
func (s *ExecutionSchedule) IsOnce() bool {
	return s.ScheduleType == ScheduleTypeOnce
}

// CalculateStatus 根据当前状态计算应设置的状态值
func (s *ExecutionSchedule) CalculateStatus() string {
	// 1. 禁用 → disabled
	if !s.Enabled {
		return ScheduleStatusDisabled
	}

	// 2. 循环调度 → running
	if s.IsCron() {
		return ScheduleStatusRunning
	}

	// 3. 单次调度
	if s.LastRunAt != nil {
		// 已执行过 → completed
		return ScheduleStatusCompleted
	}
	// 未执行 → pending
	return ScheduleStatusPending
}
