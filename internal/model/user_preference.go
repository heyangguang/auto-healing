package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// UserPreference 用户偏好设置
type UserPreference struct {
	ID          uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID      uuid.UUID       `json:"user_id" gorm:"type:uuid;not null;uniqueIndex"`
	TenantID    string          `json:"tenant_id" gorm:"type:uuid"`
	Preferences json.RawMessage `json:"preferences" gorm:"type:jsonb;default:'{}'"`
	CreatedAt   time.Time       `json:"created_at" gorm:"default:now()"`
	UpdatedAt   time.Time       `json:"updated_at" gorm:"default:now()"`
}

// TableName 表名
func (UserPreference) TableName() string {
	return "user_preferences"
}
