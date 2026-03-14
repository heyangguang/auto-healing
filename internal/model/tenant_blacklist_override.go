package model

import (
	"time"

	"github.com/google/uuid"
)

// TenantBlacklistOverride 租户对系统黑名单规则的独立开关覆盖
// 只为 is_system=true (tenant_id=NULL) 的系统规则服务
// 租户自有规则的开关直接存在 command_blacklist.is_active 上
type TenantBlacklistOverride struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID  uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`
	RuleID    uuid.UUID `json:"rule_id" gorm:"type:uuid;not null;index"`
	IsActive  bool      `json:"is_active" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt time.Time `json:"updated_at" gorm:"default:now()"`
}

func (TenantBlacklistOverride) TableName() string {
	return "tenant_blacklist_overrides"
}
