package model

import (
	"time"

	"github.com/google/uuid"
)

// 站内信分类枚举
const (
	SiteMessageCategorySystemUpdate  = "system_update"  // 系统更新
	SiteMessageCategoryFaultAlert    = "fault_alert"    // 故障通知
	SiteMessageCategoryServiceNotice = "service_notice" // 服务消息
	SiteMessageCategoryProductNews   = "product_news"   // 产品消息
	SiteMessageCategoryActivity      = "activity"       // 活动通知
	SiteMessageCategorySecurity      = "security"       // 安全公告
)

// SiteMessageCategoryInfo 分类信息
type SiteMessageCategoryInfo struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// AllSiteMessageCategories 所有分类枚举
var AllSiteMessageCategories = []SiteMessageCategoryInfo{
	{Value: SiteMessageCategorySystemUpdate, Label: "系统更新"},
	{Value: SiteMessageCategoryFaultAlert, Label: "故障通知"},
	{Value: SiteMessageCategoryServiceNotice, Label: "服务消息"},
	{Value: SiteMessageCategoryProductNews, Label: "产品消息"},
	{Value: SiteMessageCategoryActivity, Label: "活动通知"},
	{Value: SiteMessageCategorySecurity, Label: "安全公告"},
}

// SiteMessage 站内信消息主表
type SiteMessage struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Category  string     `json:"category" gorm:"type:varchar(50);not null;index"`
	Title     string     `json:"title" gorm:"type:varchar(500);not null"`
	Content   string     `json:"content" gorm:"type:text;not null"`
	CreatedAt time.Time  `json:"created_at" gorm:"default:now();index"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" gorm:"index"`
}

// TableName 表名
func (SiteMessage) TableName() string {
	return "site_messages"
}

// SiteMessageRead 站内信已读记录（懒创建：标记已读时才插入）
type SiteMessageRead struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	MessageID uuid.UUID `json:"message_id" gorm:"type:uuid;not null;uniqueIndex:idx_site_message_read_unique"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_site_message_read_unique"`
	ReadAt    time.Time `json:"read_at" gorm:"default:now()"`
}

// TableName 表名
func (SiteMessageRead) TableName() string {
	return "site_message_reads"
}

// SiteMessageSettings 站内信全局设置（单行表）
type SiteMessageSettings struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RetentionDays int       `json:"retention_days" gorm:"default:90;not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"default:now()"`
}

// TableName 表名
func (SiteMessageSettings) TableName() string {
	return "site_message_settings"
}

// SiteMessageWithReadStatus 带已读状态的站内信（查询用 DTO）
type SiteMessageWithReadStatus struct {
	ID        uuid.UUID  `json:"id"`
	Category  string     `json:"category"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	IsRead    bool       `json:"is_read"`
}
