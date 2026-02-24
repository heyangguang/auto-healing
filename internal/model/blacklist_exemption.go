package model

import (
	"time"

	"github.com/google/uuid"
)

// BlacklistExemption 安全豁免申请
type BlacklistExemption struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	TaskID        uuid.UUID  `json:"task_id" gorm:"type:uuid;not null;index"`                   // 关联任务模板
	TaskName      string     `json:"task_name" gorm:"type:varchar(256)"`                        // 冗余任务名（方便列表展示）
	RuleID        uuid.UUID  `json:"rule_id" gorm:"type:uuid;not null;index"`                   // 关联黑名单规则
	RuleName      string     `json:"rule_name" gorm:"type:varchar(128)"`                        // 冗余规则名
	RuleSeverity  string     `json:"rule_severity" gorm:"type:varchar(20)"`                     // 冗余严重级别
	RulePattern   string     `json:"rule_pattern" gorm:"type:text"`                             // 冗余匹配模式
	Reason        string     `json:"reason" gorm:"type:text;not null"`                          // 豁免原因
	RequestedBy   uuid.UUID  `json:"requested_by" gorm:"type:uuid;not null"`                    // 申请人
	RequesterName string     `json:"requester_name" gorm:"type:varchar(128)"`                   // 冗余申请人名
	Status        string     `json:"status" gorm:"type:varchar(20);not null;default:'pending'"` // pending|approved|rejected|expired
	ApprovedBy    *uuid.UUID `json:"approved_by,omitempty" gorm:"type:uuid"`                    // 审批人
	ApproverName  string     `json:"approver_name" gorm:"type:varchar(128)"`                    // 冗余审批人名
	ApprovedAt    *time.Time `json:"approved_at,omitempty"`                                     // 审批时间
	RejectReason  string     `json:"reject_reason" gorm:"type:text"`                            // 拒绝原因
	ValidityDays  int        `json:"validity_days" gorm:"not null;default:30"`                  // 有效天数
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`                                      // 到期时间（审批通过后设置）
	CreatedAt     time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"default:now()"`
}

// TableName 表名
func (BlacklistExemption) TableName() string {
	return "blacklist_exemptions"
}
