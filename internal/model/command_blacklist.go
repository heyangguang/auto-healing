package model

import (
	"time"

	"github.com/google/uuid"
)

// CommandBlacklist 高危指令黑名单规则
type CommandBlacklist struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`                     // 所属租户
	Name        string     `json:"name" gorm:"type:varchar(128);not null"`                         // 规则名称
	Pattern     string     `json:"pattern" gorm:"type:text;not null"`                              // 匹配模式
	MatchType   string     `json:"match_type" gorm:"type:varchar(20);not null;default:'contains'"` // contains|regex|exact
	Severity    string     `json:"severity" gorm:"type:varchar(20);not null;default:'critical'"`   // critical|high|medium
	Category    string     `json:"category" gorm:"type:varchar(64)"`                               // filesystem|network|system|database
	Description string     `json:"description" gorm:"type:text"`                                   // 风险说明
	IsActive    bool       `json:"is_active" gorm:"default:true"`                                  // 是否启用
	IsSystem    bool       `json:"is_system" gorm:"default:false"`                                 // 是否内置（不可删除）
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"default:now()"`
}

// TableName 表名
func (CommandBlacklist) TableName() string {
	return "command_blacklist"
}

// CommandBlacklistViolation 违规项
type CommandBlacklistViolation struct {
	File     string `json:"file"`      // 文件路径（相对于工作空间）
	Line     int    `json:"line"`      // 行号
	Content  string `json:"content"`   // 匹配到的行内容
	RuleName string `json:"rule_name"` // 触发的规则名称
	Pattern  string `json:"pattern"`   // 匹配的模式
	Severity string `json:"severity"`  // 严重级别
}
