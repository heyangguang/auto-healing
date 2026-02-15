package model

import (
	"time"

	"github.com/google/uuid"
)

// Plugin 插件模型
type Plugin struct {
	ID                  uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name                string     `json:"name" gorm:"type:varchar(100);not null;uniqueIndex"`
	Type                string     `json:"type" gorm:"type:varchar(50);not null"` // itsm, cmdb
	Description         string     `json:"description,omitempty" gorm:"type:text"`
	Version             string     `json:"version" gorm:"type:varchar(20);not null;default:'1.0.0'"`
	Config              JSON       `json:"config" gorm:"type:jsonb;not null"`
	FieldMapping        JSON       `json:"field_mapping" gorm:"type:jsonb;default:'{}'"`
	SyncFilter          JSON       `json:"sync_filter,omitempty" gorm:"type:jsonb"` // 同步过滤器配置
	SyncEnabled         bool       `json:"sync_enabled" gorm:"default:true"`
	SyncIntervalMinutes int        `json:"sync_interval_minutes" gorm:"default:5"`
	LastSyncAt          *time.Time `json:"last_sync_at,omitempty"`
	NextSyncAt          *time.Time `json:"next_sync_at,omitempty"`

	// 连续失败自动暂停
	MaxFailures         int    `json:"max_failures" gorm:"default:5"`                   // 最大连续失败次数，0=不启用自动暂停
	ConsecutiveFailures int    `json:"consecutive_failures" gorm:"default:0"`           // 当前连续失败次数
	PauseReason         string `json:"pause_reason,omitempty" gorm:"type:varchar(500)"` // 自动暂停原因

	Status       string    `json:"status" gorm:"type:varchar(20);default:'inactive'"` // active, inactive, error
	ErrorMessage string    `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"default:now()"`
}

// TableName 表名
func (Plugin) TableName() string {
	return "plugins"
}

// PluginSyncLog 插件同步日志
type PluginSyncLog struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PluginID         uuid.UUID  `json:"plugin_id" gorm:"type:uuid;not null"`
	SyncType         string     `json:"sync_type" gorm:"type:varchar(20);not null"` // scheduled, manual, webhook
	Status           string     `json:"status" gorm:"type:varchar(20);not null"`    // running, success, failed
	RecordsFetched   int        `json:"records_fetched" gorm:"default:0"`           // 从外部系统拉取的总数
	RecordsFiltered  int        `json:"records_filtered" gorm:"default:0"`          // 被过滤器筛选掉的数量
	RecordsProcessed int        `json:"records_processed" gorm:"default:0"`         // 实际处理（入库）的数量
	RecordsNew       int        `json:"records_new" gorm:"default:0"`               // 新增数量
	RecordsUpdated   int        `json:"records_updated" gorm:"default:0"`           // 更新数量
	RecordsFailed    int        `json:"records_failed" gorm:"default:0"`            // 处理失败数量
	StartedAt        time.Time  `json:"started_at" gorm:"default:now()"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	ErrorMessage     string     `json:"error_message,omitempty" gorm:"type:text"`
	Details          JSON       `json:"details,omitempty" gorm:"type:jsonb;default:'{}'"` // 详细信息（含筛选原因）

	// 关联 (不在 JSON 中输出，减少数据量)
	Plugin Plugin `json:"-" gorm:"foreignKey:PluginID"`
}

// TableName 表名
func (PluginSyncLog) TableName() string {
	return "plugin_sync_logs"
}

// Incident 工单/事件模型
type Incident struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PluginID           *uuid.UUID `json:"plugin_id" gorm:"type:uuid"`                  // 可空，插件删除后为 NULL
	SourcePluginName   string     `json:"source_plugin_name" gorm:"type:varchar(100)"` // 插件名称（插件删除后保留）
	ExternalID         string     `json:"external_id" gorm:"type:varchar(200);not null"`
	Title              string     `json:"title" gorm:"type:varchar(500);not null"`
	Description        string     `json:"description" gorm:"type:text"`
	Severity           string     `json:"severity" gorm:"type:varchar(20)"` // critical, high, medium, low
	Priority           string     `json:"priority" gorm:"type:varchar(20)"`
	Status             string     `json:"status" gorm:"type:varchar(50)"` // open, in_progress, resolved, closed
	Category           string     `json:"category" gorm:"type:varchar(100)"`
	AffectedCI         string     `json:"affected_ci" gorm:"type:varchar(200)"`
	AffectedService    string     `json:"affected_service" gorm:"type:varchar(200)"`
	Assignee           string     `json:"assignee" gorm:"type:varchar(200)"`
	Reporter           string     `json:"reporter" gorm:"type:varchar(200)"`
	RawData            JSON       `json:"raw_data" gorm:"type:jsonb;not null"`
	HealingStatus      string     `json:"healing_status" gorm:"type:varchar(50);default:'pending'"` // pending, processing, healed, failed, skipped
	WorkflowInstanceID *uuid.UUID `json:"workflow_instance_id" gorm:"type:uuid"`
	// 自愈引擎相关字段
	Scanned               bool       `json:"scanned" gorm:"default:false"`                              // 是否已被扫描
	MatchedRuleID         *uuid.UUID `json:"matched_rule_id,omitempty" gorm:"type:uuid;index"`          // 匹配的规则ID
	HealingFlowInstanceID *uuid.UUID `json:"healing_flow_instance_id,omitempty" gorm:"type:uuid;index"` // 流程实例ID
	SourceCreatedAt       *time.Time `json:"source_created_at"`
	SourceUpdatedAt       *time.Time `json:"source_updated_at"`
	CreatedAt             time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt             time.Time  `json:"updated_at" gorm:"default:now()"`

	// 关联 (不在 JSON 中输出，减少数据量)
	Plugin *Plugin `json:"-" gorm:"foreignKey:PluginID"`
}

// TableName 表名
func (Incident) TableName() string {
	return "incidents"
}
