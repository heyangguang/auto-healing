package model

import (
	"time"

	"github.com/google/uuid"
)

// ==================== Impersonation 申请 ====================
// 平台管理员需要通过审批流程才能以租户身份访问租户数据

// Impersonation 申请状态枚举
const (
	ImpersonationStatusPending   = "pending"   // 待审批
	ImpersonationStatusApproved  = "approved"  // 已批准（等待进入）
	ImpersonationStatusRejected  = "rejected"  // 已拒绝
	ImpersonationStatusActive    = "active"    // 会话进行中
	ImpersonationStatusCompleted = "completed" // 已完成（主动退出）
	ImpersonationStatusExpired   = "expired"   // 已过期（超时自动退出）
	ImpersonationStatusCancelled = "cancelled" // 已撤销（申请人主动取消）
)

// ImpersonationRequest 平台管理员访问租户的申请
type ImpersonationRequest struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RequesterID      uuid.UUID  `json:"requester_id" gorm:"type:uuid;not null"`
	RequesterName    string     `json:"requester_name" gorm:"type:varchar(200);not null"`
	TenantID         uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null"`
	TenantName       string     `json:"tenant_name" gorm:"type:varchar(200);not null"`
	Reason           string     `json:"reason,omitempty" gorm:"type:text"`
	DurationMinutes  int        `json:"duration_minutes" gorm:"not null;default:60"`
	Status           string     `json:"status" gorm:"type:varchar(20);not null;default:'pending'"`
	ApprovedBy       *uuid.UUID `json:"approved_by,omitempty" gorm:"type:uuid"`
	ApprovedAt       *time.Time `json:"approved_at,omitempty"`
	SessionStartedAt *time.Time `json:"session_started_at,omitempty"`
	SessionExpiresAt *time.Time `json:"session_expires_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"default:now()"`

	// 虚拟字段（join 查询时填充）
	ApproverName string `json:"approver_name,omitempty" gorm:"-"`
}

// TableName 表名
func (ImpersonationRequest) TableName() string {
	return "impersonation_requests"
}

// IsSessionValid 检查 Impersonation 会话是否仍然有效
func (r *ImpersonationRequest) IsSessionValid() bool {
	if r.Status != ImpersonationStatusActive {
		return false
	}
	if r.SessionExpiresAt == nil {
		return false
	}
	return time.Now().Before(*r.SessionExpiresAt)
}

// ImpersonationApprover 审批人配置（租户级）
type ImpersonationApprover struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID  uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`

	// 关联（查询时填充）
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName 表名
func (ImpersonationApprover) TableName() string {
	return "impersonation_approvers"
}
