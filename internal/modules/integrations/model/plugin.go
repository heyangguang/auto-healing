package model

import (
	"time"

	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

// Plugin 插件模型
type Plugin struct {
	ID                  uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID            *uuid.UUID      `json:"tenant_id,omitempty" gorm:"type:uuid;uniqueIndex:idx_plugin_tenant_name"`
	Name                string          `json:"name" gorm:"type:varchar(100);not null;uniqueIndex:idx_plugin_tenant_name"`
	Type                string          `json:"type" gorm:"type:varchar(50);not null"` // itsm, cmdb
	Description         string          `json:"description,omitempty" gorm:"type:text"`
	Version             string          `json:"version" gorm:"type:varchar(20);not null;default:'1.0.0'"`
	Config              modeltypes.JSON `json:"config" gorm:"type:jsonb;not null"`
	FieldMapping        modeltypes.JSON `json:"field_mapping" gorm:"type:jsonb;default:'{}'"`
	SyncFilter          modeltypes.JSON `json:"sync_filter,omitempty" gorm:"type:jsonb"` // 同步过滤器配置
	SyncEnabled         bool            `json:"sync_enabled" gorm:"default:true"`
	SyncIntervalMinutes int             `json:"sync_interval_minutes" gorm:"default:5"`
	LastSyncAt          *time.Time      `json:"last_sync_at,omitempty"`
	NextSyncAt          *time.Time      `json:"next_sync_at,omitempty"`

	MaxFailures         int    `json:"max_failures" gorm:"default:5"`
	ConsecutiveFailures int    `json:"consecutive_failures" gorm:"default:0"`
	PauseReason         string `json:"pause_reason,omitempty" gorm:"type:varchar(500)"`

	Status       string    `json:"status" gorm:"type:varchar(20);default:'inactive'"`
	ErrorMessage string    `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"default:now()"`
}

func (Plugin) TableName() string {
	return "plugins"
}

// PluginSyncLog 插件同步日志
type PluginSyncLog struct {
	ID               uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID         *uuid.UUID      `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	PluginID         uuid.UUID       `json:"plugin_id" gorm:"type:uuid;not null"`
	SyncType         string          `json:"sync_type" gorm:"type:varchar(20);not null"`
	Status           string          `json:"status" gorm:"type:varchar(20);not null"`
	RecordsFetched   int             `json:"records_fetched" gorm:"default:0"`
	RecordsFiltered  int             `json:"records_filtered" gorm:"default:0"`
	RecordsProcessed int             `json:"records_processed" gorm:"default:0"`
	RecordsNew       int             `json:"records_new" gorm:"default:0"`
	RecordsUpdated   int             `json:"records_updated" gorm:"default:0"`
	RecordsFailed    int             `json:"records_failed" gorm:"default:0"`
	StartedAt        time.Time       `json:"started_at" gorm:"default:now()"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
	ErrorMessage     string          `json:"error_message,omitempty" gorm:"type:text"`
	Details          modeltypes.JSON `json:"details,omitempty" gorm:"type:jsonb;default:'{}'"`

	Plugin Plugin `json:"-" gorm:"foreignKey:PluginID"`
}

func (PluginSyncLog) TableName() string {
	return "plugin_sync_logs"
}
