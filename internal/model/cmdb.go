package model

import (
	"time"

	"github.com/google/uuid"
)

// CMDBItem CMDB 配置项模型
type CMDBItem struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PluginID         *uuid.UUID `json:"plugin_id" gorm:"type:uuid"`                  // 可空，插件删除后为 NULL
	SourcePluginName string     `json:"source_plugin_name" gorm:"type:varchar(100)"` // 插件名称（插件删除后保留）
	ExternalID       string     `json:"external_id" gorm:"type:varchar(200);not null"`
	Name             string     `json:"name" gorm:"type:varchar(200);not null"`
	Type             string     `json:"type" gorm:"type:varchar(50)"`   // server, application, network, database
	Status           string     `json:"status" gorm:"type:varchar(50)"` // active, offline, maintenance
	IPAddress        string     `json:"ip_address" gorm:"type:varchar(50)"`
	Hostname         string     `json:"hostname" gorm:"type:varchar(200)"`
	OS               string     `json:"os" gorm:"type:varchar(100)"`            // Linux, Windows, etc.
	OSVersion        string     `json:"os_version" gorm:"type:varchar(100)"`    // CentOS 7.9, Ubuntu 22.04, etc.
	CPU              string     `json:"cpu" gorm:"type:varchar(100)"`           // 4 vCPU, Intel Xeon, etc.
	Memory           string     `json:"memory" gorm:"type:varchar(50)"`         // 16GB, 32GB, etc.
	Disk             string     `json:"disk" gorm:"type:varchar(100)"`          // 500GB SSD, etc.
	Location         string     `json:"location" gorm:"type:varchar(200)"`      // 机房位置
	Owner            string     `json:"owner" gorm:"type:varchar(200)"`         // 负责人
	Environment      string     `json:"environment" gorm:"type:varchar(50)"`    // prod, test, dev
	Manufacturer     string     `json:"manufacturer" gorm:"type:varchar(100)"`  // Dell, HP, etc.
	Model            string     `json:"model" gorm:"type:varchar(100)"`         // PowerEdge R740, etc.
	SerialNumber     string     `json:"serial_number" gorm:"type:varchar(100)"` // 序列号
	Department       string     `json:"department" gorm:"type:varchar(100)"`    // 所属部门
	Dependencies     JSONArray  `json:"dependencies" gorm:"type:jsonb;default:'[]'"`
	Tags             JSON       `json:"tags" gorm:"type:jsonb;default:'{}'"`
	RawData          JSON       `json:"raw_data" gorm:"type:jsonb;not null"`
	SourceCreatedAt  *time.Time `json:"source_created_at"`
	SourceUpdatedAt  *time.Time `json:"source_updated_at"`
	CreatedAt        time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"default:now()"`

	// 维护模式相关
	MaintenanceReason  string     `json:"maintenance_reason,omitempty" gorm:"type:varchar(500)"`
	MaintenanceStartAt *time.Time `json:"maintenance_start_at,omitempty"`
	MaintenanceEndAt   *time.Time `json:"maintenance_end_at,omitempty"`

	// 关联 (不在 JSON 中输出)
	Plugin *Plugin `json:"-" gorm:"foreignKey:PluginID"`
}

// TableName 表名
func (CMDBItem) TableName() string {
	return "cmdb_items"
}

// CMDBMaintenanceLog CMDB 维护日志
type CMDBMaintenanceLog struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CMDBItemID     uuid.UUID  `json:"cmdb_item_id" gorm:"type:uuid;not null;index"`
	CMDBItemName   string     `json:"cmdb_item_name" gorm:"type:varchar(200)"`
	Action         string     `json:"action" gorm:"type:varchar(20);not null"` // enter, exit
	Reason         string     `json:"reason" gorm:"type:varchar(500)"`
	ScheduledEndAt *time.Time `json:"scheduled_end_at,omitempty"`
	ActualEndAt    *time.Time `json:"actual_end_at,omitempty"`
	ExitType       string     `json:"exit_type,omitempty" gorm:"type:varchar(20)"` // manual, auto（手动恢复/自动恢复）
	Operator       string     `json:"operator" gorm:"type:varchar(100)"`
	CreatedAt      time.Time  `json:"created_at" gorm:"default:now()"`
}

// TableName 表名
func (CMDBMaintenanceLog) TableName() string {
	return "cmdb_maintenance_logs"
}
