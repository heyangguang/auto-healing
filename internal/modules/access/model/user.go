package model

import (
	"time"

	"github.com/google/uuid"
)

// User 用户模型
type User struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Username          string     `json:"username" gorm:"type:varchar(100);not null;uniqueIndex"`
	Email             string     `json:"email" gorm:"type:varchar(200);not null;uniqueIndex"`
	PasswordHash      string     `json:"-" gorm:"type:varchar(200);not null"`
	DisplayName       string     `json:"display_name" gorm:"type:varchar(200)"`
	Phone             string     `json:"phone,omitempty" gorm:"type:varchar(50)"`
	AvatarURL         string     `json:"avatar_url,omitempty" gorm:"type:varchar(500)"`
	Status            string     `json:"status" gorm:"type:varchar(20);default:'active'"`
	LastLoginAt       *time.Time `json:"last_login_at,omitempty"`
	LastLoginIP       string     `json:"last_login_ip,omitempty" gorm:"type:varchar(45)"`
	PasswordChangedAt time.Time  `json:"-" gorm:"default:now()"`
	FailedLoginCount  int        `json:"-" gorm:"default:0"`
	LockedUntil       *time.Time `json:"-"`
	CreatedAt         time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"default:now()"`
	IsPlatformAdmin   bool       `json:"is_platform_admin" gorm:"default:false"` // 平台管理员标识

	// 关联
	Roles []Role `json:"roles,omitempty" gorm:"many2many:user_platform_roles;"`
}

// TableName 表名
func (User) TableName() string {
	return "users"
}

// Role 角色模型
type Role struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string     `json:"name" gorm:"type:varchar(100);not null;uniqueIndex"`
	DisplayName string     `json:"display_name" gorm:"type:varchar(200);not null"`
	Description string     `json:"description,omitempty" gorm:"type:text"`
	IsSystem    bool       `json:"is_system" gorm:"default:false"`
	Scope       string     `json:"scope" gorm:"type:varchar(20);default:'tenant'"` // platform=平台级, tenant=租户级
	TenantID    *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid"`           // NULL=系统模板, UUID=租户自定义
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"default:now()"`

	// 关联
	Permissions []Permission `json:"permissions,omitempty" gorm:"many2many:role_permissions;"`
}

// TableName 表名
func (Role) TableName() string {
	return "roles"
}

// Permission 权限模型
type Permission struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Code        string    `json:"code" gorm:"type:varchar(200);not null;uniqueIndex"`
	Name        string    `json:"name" gorm:"type:varchar(200);not null"`
	Description string    `json:"description,omitempty" gorm:"type:text"`
	Module      string    `json:"module" gorm:"type:varchar(50);not null"`
	Resource    string    `json:"resource" gorm:"type:varchar(50);not null"`
	Action      string    `json:"action" gorm:"type:varchar(50);not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"default:now()"`
}

// TableName 表名
func (Permission) TableName() string {
	return "permissions"
}

// UserPlatformRole 用户平台角色关联（平台级全局角色）
type UserPlatformRole struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	RoleID    uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt time.Time `gorm:"default:now()"`
}

// TableName 表名
func (UserPlatformRole) TableName() string {
	return "user_platform_roles"
}

// RolePermission 角色权限关联
type RolePermission struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RoleID       uuid.UUID `gorm:"type:uuid;not null"`
	PermissionID uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt    time.Time `gorm:"default:now()"`
}

// TableName 表名
func (RolePermission) TableName() string {
	return "role_permissions"
}

// RefreshToken 刷新令牌
type RefreshToken struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID     uuid.UUID `gorm:"type:uuid;not null"`
	TokenHash  string    `gorm:"type:varchar(200);not null;uniqueIndex"`
	DeviceInfo string    `gorm:"type:jsonb"`
	ExpiredAt  time.Time `gorm:"not null"`
	CreatedAt  time.Time `gorm:"default:now()"`
}

// TableName 表名
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// TokenBlacklist Token黑名单
type TokenBlacklist struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TokenJTI  string     `gorm:"type:varchar(100);not null;uniqueIndex"`
	UserID    *uuid.UUID `gorm:"type:uuid"`
	ExpiredAt time.Time  `gorm:"not null"`
	CreatedAt time.Time  `gorm:"default:now()"`
}

// TableName 表名
func (TokenBlacklist) TableName() string {
	return "token_blacklist"
}
