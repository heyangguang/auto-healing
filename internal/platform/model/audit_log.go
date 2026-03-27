package model

import (
	"time"

	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

// AuditLog 租户级审计日志模型
type AuditLog struct {
	ID             uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       *uuid.UUID      `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	UserID         *uuid.UUID      `json:"user_id,omitempty" gorm:"type:uuid"`
	Username       string          `json:"username,omitempty" gorm:"type:varchar(200)"`
	IPAddress      string          `json:"ip_address,omitempty" gorm:"type:varchar(45)"`
	UserAgent      string          `json:"user_agent,omitempty" gorm:"type:text"`
	Category       string          `json:"category" gorm:"type:varchar(20);not null;default:'operation'"`
	Action         string          `json:"action" gorm:"type:varchar(100);not null"`
	ResourceType   string          `json:"resource_type" gorm:"type:varchar(100);not null"`
	ResourceID     *uuid.UUID      `json:"resource_id,omitempty" gorm:"type:uuid"`
	ResourceName   string          `json:"resource_name,omitempty" gorm:"type:varchar(200)"`
	RequestMethod  string          `json:"request_method,omitempty" gorm:"type:varchar(10)"`
	RequestPath    string          `json:"request_path,omitempty" gorm:"type:varchar(500)"`
	RequestBody    modeltypes.JSON `json:"request_body,omitempty" gorm:"type:jsonb"`
	ResponseStatus *int            `json:"response_status,omitempty"`
	Changes        modeltypes.JSON `json:"changes,omitempty" gorm:"type:jsonb"`
	Status         string          `json:"status" gorm:"type:varchar(20);not null"`
	ErrorMessage   string          `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt      time.Time       `json:"created_at" gorm:"default:now()"`

	User *accessmodel.User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

// PlatformAuditLog 平台级审计日志模型
type PlatformAuditLog struct {
	ID             uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID         *uuid.UUID      `json:"user_id,omitempty" gorm:"type:uuid"`
	Username       string          `json:"username,omitempty" gorm:"type:varchar(200)"`
	IPAddress      string          `json:"ip_address,omitempty" gorm:"type:varchar(45)"`
	UserAgent      string          `json:"user_agent,omitempty" gorm:"type:text"`
	Category       string          `json:"category" gorm:"type:varchar(20);not null;default:'operation'"`
	Action         string          `json:"action" gorm:"type:varchar(100);not null"`
	ResourceType   string          `json:"resource_type" gorm:"type:varchar(100);not null"`
	ResourceID     *uuid.UUID      `json:"resource_id,omitempty" gorm:"type:uuid"`
	ResourceName   string          `json:"resource_name,omitempty" gorm:"type:varchar(200)"`
	RequestMethod  string          `json:"request_method,omitempty" gorm:"type:varchar(10)"`
	RequestPath    string          `json:"request_path,omitempty" gorm:"type:varchar(500)"`
	RequestBody    modeltypes.JSON `json:"request_body,omitempty" gorm:"type:jsonb"`
	ResponseStatus *int            `json:"response_status,omitempty"`
	Changes        modeltypes.JSON `json:"changes,omitempty" gorm:"type:jsonb"`
	Status         string          `json:"status" gorm:"type:varchar(20);not null"`
	ErrorMessage   string          `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt      time.Time       `json:"created_at" gorm:"default:now()"`
}

func (PlatformAuditLog) TableName() string {
	return "platform_audit_logs"
}
