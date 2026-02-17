package model

import (
	"time"

	"github.com/google/uuid"
)

// UserFavorite 用户收藏菜单项
type UserFavorite struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_user_favorite"`
	MenuKey   string    `json:"menu_key" gorm:"type:varchar(200);not null;uniqueIndex:idx_user_favorite"`
	Name      string    `json:"name" gorm:"type:varchar(200);not null"`
	Path      string    `json:"path" gorm:"type:varchar(500);not null"`
	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`
}

// TableName 表名
func (UserFavorite) TableName() string {
	return "user_favorites"
}

// UserRecent 用户最近访问记录
type UserRecent struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID     uuid.UUID `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_user_recent"`
	MenuKey    string    `json:"menu_key" gorm:"type:varchar(200);not null;uniqueIndex:idx_user_recent"`
	Name       string    `json:"name" gorm:"type:varchar(200);not null"`
	Path       string    `json:"path" gorm:"type:varchar(500);not null"`
	AccessedAt time.Time `json:"accessed_at" gorm:"default:now()"`
}

// TableName 表名
func (UserRecent) TableName() string {
	return "user_recents"
}
