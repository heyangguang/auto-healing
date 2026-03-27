package model

import (
	"time"

	integrationsmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

// CMDBItem CMDB 配置项模型
type CMDBItem struct {
	ID                 uuid.UUID            `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID           `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	PluginID           *uuid.UUID           `json:"plugin_id" gorm:"type:uuid"`
	SourcePluginName   string               `json:"source_plugin_name" gorm:"type:varchar(100)"`
	ExternalID         string               `json:"external_id" gorm:"type:varchar(200);not null"`
	Name               string               `json:"name" gorm:"type:varchar(200);not null"`
	Type               string               `json:"type" gorm:"type:varchar(50)"`
	Status             string               `json:"status" gorm:"type:varchar(50)"`
	IPAddress          string               `json:"ip_address" gorm:"type:varchar(50)"`
	Hostname           string               `json:"hostname" gorm:"type:varchar(200)"`
	OS                 string               `json:"os" gorm:"type:varchar(100)"`
	OSVersion          string               `json:"os_version" gorm:"type:varchar(100)"`
	CPU                string               `json:"cpu" gorm:"type:varchar(100)"`
	Memory             string               `json:"memory" gorm:"type:varchar(50)"`
	Disk               string               `json:"disk" gorm:"type:varchar(100)"`
	Location           string               `json:"location" gorm:"type:varchar(200)"`
	Owner              string               `json:"owner" gorm:"type:varchar(200)"`
	Environment        string               `json:"environment" gorm:"type:varchar(50)"`
	Manufacturer       string               `json:"manufacturer" gorm:"type:varchar(100)"`
	Model              string               `json:"model" gorm:"type:varchar(100)"`
	SerialNumber       string               `json:"serial_number" gorm:"type:varchar(100)"`
	Department         string               `json:"department" gorm:"type:varchar(100)"`
	Dependencies       modeltypes.JSONArray `json:"dependencies" gorm:"type:jsonb;default:'[]'"`
	Tags               modeltypes.JSON      `json:"tags" gorm:"type:jsonb;default:'{}'"`
	RawData            modeltypes.JSON      `json:"raw_data" gorm:"type:jsonb;not null"`
	SourceCreatedAt    *time.Time           `json:"source_created_at"`
	SourceUpdatedAt    *time.Time           `json:"source_updated_at"`
	CreatedAt          time.Time            `json:"created_at" gorm:"default:now()"`
	UpdatedAt          time.Time            `json:"updated_at" gorm:"default:now()"`
	MaintenanceReason  string               `json:"maintenance_reason,omitempty" gorm:"type:varchar(500)"`
	MaintenanceStartAt *time.Time           `json:"maintenance_start_at,omitempty"`
	MaintenanceEndAt   *time.Time           `json:"maintenance_end_at,omitempty"`

	Plugin *integrationsmodel.Plugin `json:"-" gorm:"foreignKey:PluginID"`
}

func (CMDBItem) TableName() string {
	return "cmdb_items"
}

// CMDBMaintenanceLog CMDB 维护日志
type CMDBMaintenanceLog struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	CMDBItemID     uuid.UUID  `json:"cmdb_item_id" gorm:"type:uuid;not null;index"`
	CMDBItemName   string     `json:"cmdb_item_name" gorm:"type:varchar(200)"`
	Action         string     `json:"action" gorm:"type:varchar(20);not null"`
	Reason         string     `json:"reason" gorm:"type:varchar(500)"`
	ScheduledEndAt *time.Time `json:"scheduled_end_at,omitempty"`
	ActualEndAt    *time.Time `json:"actual_end_at,omitempty"`
	ExitType       string     `json:"exit_type,omitempty" gorm:"type:varchar(20)"`
	Operator       string     `json:"operator" gorm:"type:varchar(100)"`
	CreatedAt      time.Time  `json:"created_at" gorm:"default:now()"`
}

func (CMDBMaintenanceLog) TableName() string {
	return "cmdb_maintenance_logs"
}
