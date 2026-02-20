package model

import (
	"time"

	"github.com/google/uuid"
)

// ==================== 平台级设置（Platform Settings） ====================
// 通用 KV 设置表，存储平台级配置。
// 与租户无关，所有租户共享同一份配置。
// 新增设置只需 INSERT 一行，无需改代码。

// 设置值类型
const (
	SettingTypeInt    = "int"    // 整数值
	SettingTypeString = "string" // 字符串值
	SettingTypeBool   = "bool"   // 布尔值
	SettingTypeJSON   = "json"   // JSON 对象
)

// PlatformSetting 平台级 KV 设置
type PlatformSetting struct {
	Key          string     `json:"key" gorm:"type:varchar(100);primaryKey"`                // 唯一标识，格式：module.setting_name
	Value        string     `json:"value" gorm:"type:text;not null"`                        // 当前值（统一存为字符串）
	Type         string     `json:"type" gorm:"type:varchar(20);not null;default:'string'"` // 值类型
	Module       string     `json:"module" gorm:"type:varchar(50);not null;index"`          // 所属模块（分组用）
	Label        string     `json:"label" gorm:"type:varchar(200);not null"`                // 中文名称
	Description  string     `json:"description,omitempty" gorm:"type:text"`                 // 描述/帮助文本
	DefaultValue string     `json:"default_value,omitempty" gorm:"type:text"`               // 默认值
	UpdatedAt    time.Time  `json:"updated_at" gorm:"default:now()"`                        // 最后修改时间
	UpdatedBy    *uuid.UUID `json:"updated_by,omitempty" gorm:"type:uuid"`                  // 最后修改人
}

// TableName 表名
func (PlatformSetting) TableName() string {
	return "platform_settings"
}
