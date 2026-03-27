package model

import (
	"time"

	"github.com/google/uuid"
)

// Dictionary 字典值模型
type Dictionary struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DictType  string    `json:"dict_type" gorm:"type:varchar(64);not null;uniqueIndex:idx_dict_type_key"`
	DictKey   string    `json:"dict_key" gorm:"type:varchar(64);not null;uniqueIndex:idx_dict_type_key"`
	Label     string    `json:"label" gorm:"type:varchar(128);not null"`
	LabelEn   string    `json:"label_en,omitempty" gorm:"type:varchar(128)"`
	Color     string    `json:"color,omitempty" gorm:"type:varchar(32)"`
	TagColor  string    `json:"tag_color,omitempty" gorm:"type:varchar(32)"`
	Badge     string    `json:"badge,omitempty" gorm:"type:varchar(32)"`
	Icon      string    `json:"icon,omitempty" gorm:"type:varchar(64)"`
	Bg        string    `json:"bg,omitempty" gorm:"type:varchar(32)"`
	Extra     JSON      `json:"extra,omitempty" gorm:"type:jsonb"`
	SortOrder int       `json:"sort_order" gorm:"default:0"`
	IsSystem  bool      `json:"is_system" gorm:"default:false"`
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt time.Time `json:"updated_at" gorm:"default:now()"`
}

// TableName 表名
func (Dictionary) TableName() string {
	return "sys_dictionaries"
}
