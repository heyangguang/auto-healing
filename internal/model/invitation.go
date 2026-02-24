package model

import (
	"time"

	"github.com/google/uuid"
)

// ==================== 租户邀请（Tenant Invitation） ====================
// 平台管理员邀请用户加入租户，支持邮件邀请和链接邀请两种方式。

// 邀请状态
const (
	InvitationStatusPending   = "pending"   // 待接受
	InvitationStatusAccepted  = "accepted"  // 已接受（已注册）
	InvitationStatusExpired   = "expired"   // 已过期
	InvitationStatusCancelled = "cancelled" // 已取消
)

// TenantInvitation 租户邀请记录
type TenantInvitation struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID   uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Email      string     `json:"email" gorm:"type:varchar(255);not null"`
	RoleID     uuid.UUID  `json:"role_id" gorm:"type:uuid;not null"`
	Token      string     `json:"-" gorm:"type:varchar(255)"` // 原始 token，不暴露到 JSON
	TokenHash  string     `json:"-" gorm:"type:varchar(255);not null;uniqueIndex"`
	Status     string     `json:"status" gorm:"type:varchar(20);default:'pending'"`
	InvitedBy  uuid.UUID  `json:"invited_by" gorm:"type:uuid;not null"`
	ExpiresAt  time.Time  `json:"expires_at" gorm:"not null"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"default:now()"`

	// 动态字段（不存数据库）
	InvitationURL string `json:"invitation_url,omitempty" gorm:"-"`

	// 关联（查询时手动填充，不参与 GORM 迁移）
	Tenant  *Tenant `json:"tenant,omitempty" gorm:"-"`
	Role    *Role   `json:"role,omitempty" gorm:"-"`
	Inviter *User   `json:"inviter,omitempty" gorm:"-"`
}

// TableName 表名
func (TenantInvitation) TableName() string {
	return "tenant_invitations"
}
