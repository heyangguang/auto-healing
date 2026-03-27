package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

// NotificationChannel 通知渠道模型
type NotificationChannel struct {
	ID                 uuid.UUID    `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID   `json:"tenant_id,omitempty" gorm:"type:uuid;uniqueIndex:idx_channel_tenant_name"`
	Name               string       `json:"name" gorm:"type:varchar(200);not null;uniqueIndex:idx_channel_tenant_name"`
	Type               string       `json:"type" gorm:"type:varchar(50);not null"` // email, dingtalk, webhook
	Description        string       `json:"description,omitempty" gorm:"type:text"`
	Config             JSON         `json:"-" gorm:"type:jsonb;not null"`              // 加密存储敏感信息
	RetryConfig        *RetryConfig `json:"retry_config,omitempty" gorm:"type:jsonb"`  // 重试配置
	Recipients         StringArray  `json:"recipients" gorm:"type:jsonb;default:'[]'"` // 接收人列表
	IsActive           bool         `json:"is_active" gorm:"default:true"`
	IsDefault          bool         `json:"is_default" gorm:"default:false"`
	RateLimitPerMinute *int         `json:"rate_limit_per_minute,omitempty"`
	CreatedAt          time.Time    `json:"created_at" gorm:"default:now()"`
	UpdatedAt          time.Time    `json:"updated_at" gorm:"default:now()"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries     int   `json:"max_retries"`     // 最大重试次数，默认 3
	RetryIntervals []int `json:"retry_intervals"` // 重试间隔（分钟），如 [1, 5, 15]
}

// Scan 实现 sql.Scanner 接口
func (r *RetryConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal RetryConfig: %v", value)
	}
	return json.Unmarshal(bytes, r)
}

// Value 实现 driver.Valuer 接口
func (r RetryConfig) Value() (driver.Value, error) {
	if r.MaxRetries == 0 && len(r.RetryIntervals) == 0 {
		return nil, nil
	}
	return json.Marshal(r)
}

// TableName 表名
func (NotificationChannel) TableName() string {
	return "notification_channels"
}

// NotificationTemplate 通知模板模型
type NotificationTemplate struct {
	ID                 uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID  `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	Name               string      `json:"name" gorm:"type:varchar(200);not null"`
	Description        string      `json:"description,omitempty" gorm:"type:text"`
	EventType          string      `json:"event_type,omitempty" gorm:"type:varchar(50)"` // incident_created, incident_resolved, approval_required, execution_result, custom
	SupportedChannels  StringArray `json:"supported_channels" gorm:"type:jsonb;default:'[]'"`
	SubjectTemplate    string      `json:"subject_template,omitempty" gorm:"type:text"`
	BodyTemplate       string      `json:"body_template" gorm:"type:text;not null"`
	Format             string      `json:"format" gorm:"type:varchar(20);default:'text'"` // text, markdown, html
	AvailableVariables JSONArray   `json:"available_variables" gorm:"type:jsonb;default:'[]'"`
	IsActive           bool        `json:"is_active" gorm:"default:true"`
	CreatedAt          time.Time   `json:"created_at" gorm:"default:now()"`
	UpdatedAt          time.Time   `json:"updated_at" gorm:"default:now()"`
}

// TableName 表名
func (NotificationTemplate) TableName() string {
	return "notification_templates"
}

// NotificationLog 通知日志模型
type NotificationLog struct {
	ID                 uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID  `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	TemplateID         *uuid.UUID  `json:"template_id,omitempty" gorm:"type:uuid"`
	ChannelID          uuid.UUID   `json:"channel_id" gorm:"type:uuid;not null"`
	ExecutionRunID     *uuid.UUID  `json:"execution_run_id,omitempty" gorm:"type:uuid"` // 关联执行记录
	WorkflowInstanceID *uuid.UUID  `json:"workflow_instance_id,omitempty" gorm:"type:uuid"`
	IncidentID         *uuid.UUID  `json:"incident_id,omitempty" gorm:"type:uuid"`
	Recipients         StringArray `json:"recipients" gorm:"type:jsonb;not null"`
	Subject            string      `json:"subject,omitempty" gorm:"type:text"`
	Body               string      `json:"body" gorm:"type:text;not null"`
	Status             string      `json:"status" gorm:"type:varchar(50);default:'pending'"` // pending, sent, delivered, failed, bounced
	ExternalMessageID  string      `json:"external_message_id,omitempty" gorm:"type:varchar(200)"`
	ResponseData       JSON        `json:"response_data,omitempty" gorm:"type:jsonb"`
	ErrorMessage       string      `json:"error_message,omitempty" gorm:"type:text"`
	RetryCount         int         `json:"retry_count" gorm:"default:0"`
	NextRetryAt        *time.Time  `json:"next_retry_at,omitempty"`
	SentAt             *time.Time  `json:"sent_at,omitempty"`
	CreatedAt          time.Time   `json:"created_at" gorm:"default:now()"`

	// 关联
	Template         *NotificationTemplate        `json:"template,omitempty" gorm:"foreignKey:TemplateID"`
	Channel          *NotificationChannel         `json:"channel,omitempty" gorm:"foreignKey:ChannelID"`
	ExecutionRun     *projection.ExecutionRun     `json:"execution_run,omitempty" gorm:"foreignKey:ExecutionRunID"`
	WorkflowInstance *projection.WorkflowInstance `json:"workflow_instance,omitempty" gorm:"foreignKey:WorkflowInstanceID"`
	Incident         *projection.Incident         `json:"incident,omitempty" gorm:"foreignKey:IncidentID"`
}

// TableName 表名
func (NotificationLog) TableName() string {
	return "notification_logs"
}

type NotificationTriggerConfig = modeltypes.NotificationTriggerConfig
type TaskNotificationConfig = modeltypes.TaskNotificationConfig
