package model

import (
	"time"

	"github.com/google/uuid"
)

// ==================== 租户（Tenants） ====================
// 多租户架构的核心表。
// 平台级数据（platform_settings, site_messages 等）不关联租户。
// 租户级数据（plugins, rules, flows 等）通过 tenant_id 列关联。

// 租户状态枚举
const (
	TenantStatusActive   = "active"   // 正常
	TenantStatusDisabled = "disabled" // 已禁用
)

// 默认租户 ID（所有现有数据归入此租户）
var DefaultTenantID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// Tenant 租户
type Tenant struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string    `json:"name" gorm:"type:varchar(200);not null"`                   // 显示名称
	Code        string    `json:"code" gorm:"type:varchar(50);not null;uniqueIndex"`        // 唯一编码（英文，用于 URL/API）
	Description string    `json:"description,omitempty" gorm:"type:text"`                   // 描述
	Icon        string    `json:"icon,omitempty" gorm:"type:varchar(50)"`                   // 图标名称（bank, shop, team, cloud 等）
	Status      string    `json:"status" gorm:"type:varchar(20);not null;default:'active'"` // active / disabled
	CreatedAt   time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"default:now()"`
	MemberCount int64     `json:"member_count" gorm:"-"` // 虚拟字段，通过子查询计算，不持久化
}

// TableName 表名
func (Tenant) TableName() string {
	return "tenants"
}

// UserTenantRole 用户-租户-角色关联（替代 user_roles）
// 一个用户可以在多个租户中拥有不同角色
type UserTenantRole struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	TenantID  uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`
	RoleID    uuid.UUID `json:"role_id" gorm:"type:uuid;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`

	// 关联
	Tenant Tenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Role   Role   `json:"role,omitempty" gorm:"foreignKey:RoleID"`
	User   User   `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName 表名
func (UserTenantRole) TableName() string {
	return "user_tenant_roles"
}
