package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationRepository 通知相关的数据访问层
type NotificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository 创建通知仓库
func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// TemplateListOptions 模板列表查询选项
type TemplateListOptions struct {
	Page             int
	PageSize         int
	Name             query.StringFilter // 模糊搜索模板名称
	EventType        string             // 事件类型筛选
	IsActive         *bool              // 按启用状态筛选
	Format           string             // 按格式筛选
	SupportedChannel string             // 按支持渠道筛选
	SortBy           string             // 排序字段
	SortOrder        string             // 排序方向
}

// NotificationLogListOptions 通知日志列表查询选项
type NotificationLogListOptions struct {
	Page           int
	PageSize       int
	Status         string             // 状态筛选
	ChannelID      *uuid.UUID         // 渠道 ID
	TemplateID     *uuid.UUID         // 模板 ID
	TaskID         *uuid.UUID         // 任务模板 ID
	TaskName       query.StringFilter // 任务模板名称（搜索）
	TriggeredBy    string             // 触发类型: manual, scheduler:cron, scheduler:once, healing
	ExecutionRunID *uuid.UUID         // 执行记录 ID
	Subject        query.StringFilter // 搜索主题
	CreatedAfter   *time.Time         // 创建时间起始
	CreatedBefore  *time.Time         // 创建时间结束
	SortBy         string             // 排序字段
	SortOrder      string             // 排序方向
}

func (r *NotificationRepository) tenantDB(ctx context.Context) *gorm.DB {
	return TenantDB(r.db, ctx)
}
