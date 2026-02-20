package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
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

// ==================== 渠道管理 ====================

// CreateChannel 创建通知渠道
func (r *NotificationRepository) CreateChannel(ctx context.Context, channel *model.NotificationChannel) error {
	if channel.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		channel.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(channel).Error
}

// GetChannelByID 根据 ID 获取渠道
func (r *NotificationRepository) GetChannelByID(ctx context.Context, id uuid.UUID) (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := TenantDB(r.db, ctx).Where("id = ?", id).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetChannelByName 根据名称获取渠道
func (r *NotificationRepository) GetChannelByName(ctx context.Context, name string) (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := TenantDB(r.db, ctx).Where("name = ?", name).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// ListChannels 获取渠道列表
func (r *NotificationRepository) ListChannels(ctx context.Context, page, pageSize int, channelType string, search string) ([]model.NotificationChannel, int64, error) {
	var channels []model.NotificationChannel
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.NotificationChannel{})
	if channelType != "" {
		query = query.Where("type = ?", channelType)
	}
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("(name ILIKE ? OR description ILIKE ?)", pattern, pattern)
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, 0, err
	}

	return channels, total, nil
}

// UpdateChannel 更新渠道
func (r *NotificationRepository) UpdateChannel(ctx context.Context, channel *model.NotificationChannel) error {
	return r.db.WithContext(ctx).Save(channel).Error
}

// DeleteChannel 删除渠道
func (r *NotificationRepository) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Delete(&model.NotificationChannel{}, "id = ?", id).Error
}

// GetDefaultChannel 获取默认渠道
func (r *NotificationRepository) GetDefaultChannel(ctx context.Context) (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := TenantDB(r.db, ctx).Where("is_default = ? AND is_active = ?", true, true).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetChannelsByIDs 批量获取渠道
func (r *NotificationRepository) GetChannelsByIDs(ctx context.Context, ids []uuid.UUID) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	if err := TenantDB(r.db, ctx).Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// CountTemplatesUsingChannelType 统计使用指定渠道类型的模板数量
func (r *NotificationRepository) CountTemplatesUsingChannelType(ctx context.Context, channelType string) (int64, error) {
	var count int64
	// 检查 supported_channels 数组是否包含该渠道类型
	err := TenantDB(r.db, ctx).Model(&model.NotificationTemplate{}).
		Where("supported_channels @> ?", `["`+channelType+`"]`).
		Count(&count).Error
	return count, err
}

// CountTasksUsingTemplate 统计使用指定模板的任务模板数量
func (r *NotificationRepository) CountTasksUsingTemplate(ctx context.Context, templateID uuid.UUID) (int64, error) {
	var count int64
	// 检查 notification_config.template_id 是否等于该模板 ID
	err := TenantDB(r.db, ctx).Model(&model.ExecutionTask{}).
		Where("notification_config->>'template_id' = ?", templateID.String()).
		Count(&count).Error
	return count, err
}

// CountTasksUsingChannel 统计使用指定渠道的任务模板数量
func (r *NotificationRepository) CountTasksUsingChannel(ctx context.Context, channelID uuid.UUID) (int64, error) {
	var count int64
	// 检查 notification_config.channel_ids 数组是否包含该渠道 ID
	err := TenantDB(r.db, ctx).Model(&model.ExecutionTask{}).
		Where("notification_config->'channel_ids' @> ?", `["`+channelID.String()+`"]`).
		Count(&count).Error
	return count, err
}

// ==================== 模板管理 ====================

// CreateTemplate 创建通知模板
func (r *NotificationRepository) CreateTemplate(ctx context.Context, template *model.NotificationTemplate) error {
	if template.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		template.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(template).Error
}

// GetTemplateByID 根据 ID 获取模板
func (r *NotificationRepository) GetTemplateByID(ctx context.Context, id uuid.UUID) (*model.NotificationTemplate, error) {
	var template model.NotificationTemplate
	if err := TenantDB(r.db, ctx).Where("id = ?", id).First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

// GetTemplatesByIDs 批量获取模板
func (r *NotificationRepository) GetTemplatesByIDs(ctx context.Context, ids []uuid.UUID) ([]model.NotificationTemplate, error) {
	var templates []model.NotificationTemplate
	if err := TenantDB(r.db, ctx).Where("id IN ?", ids).Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// TemplateListOptions 模板列表查询选项
type TemplateListOptions struct {
	Page             int
	PageSize         int
	Search           string // 模糊搜索模板名称
	EventType        string // 事件类型筛选
	IsActive         *bool  // 按启用状态筛选
	Format           string // 按格式筛选
	SupportedChannel string // 按支持渠道筛选
	SortBy           string // 排序字段
	SortOrder        string // 排序方向
}

// ListTemplates 获取模板列表
func (r *NotificationRepository) ListTemplates(ctx context.Context, opts *TemplateListOptions) ([]model.NotificationTemplate, int64, error) {
	var templates []model.NotificationTemplate
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.NotificationTemplate{})

	// 模糊搜索模板名称
	if opts.Search != "" {
		query = query.Where("name ILIKE ?", "%"+opts.Search+"%")
	}

	// 事件类型筛选
	if opts.EventType != "" {
		query = query.Where("event_type = ?", opts.EventType)
	}

	// 按启用状态筛选
	if opts.IsActive != nil {
		query = query.Where("is_active = ?", *opts.IsActive)
	}

	// 按格式筛选
	if opts.Format != "" {
		query = query.Where("format = ?", opts.Format)
	}

	// 按支持渠道筛选
	if opts.SupportedChannel != "" {
		query = query.Where("supported_channels @> ?", `["`+opts.SupportedChannel+`"]`)
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	orderClause := "created_at DESC"
	if opts.SortBy != "" {
		order := "ASC"
		if opts.SortOrder == "desc" {
			order = "DESC"
		}
		// 白名单验证排序字段
		allowedSortFields := map[string]bool{
			"name": true, "created_at": true, "updated_at": true, "format": true, "event_type": true,
		}
		if allowedSortFields[opts.SortBy] {
			orderClause = opts.SortBy + " " + order
		}
	}

	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Offset(offset).Limit(opts.PageSize).Order(orderClause).Find(&templates).Error; err != nil {
		return nil, 0, err
	}

	return templates, total, nil
}

// UpdateTemplate 更新模板
func (r *NotificationRepository) UpdateTemplate(ctx context.Context, template *model.NotificationTemplate) error {
	return r.db.WithContext(ctx).Save(template).Error
}

// DeleteTemplate 删除模板
func (r *NotificationRepository) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Delete(&model.NotificationTemplate{}, "id = ?", id).Error
}

// ==================== 通知日志管理 ====================

// CreateLog 创建通知日志
func (r *NotificationRepository) CreateLog(ctx context.Context, log *model.NotificationLog) error {
	if log.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		log.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// GetLogByID 根据 ID 获取日志
func (r *NotificationRepository) GetLogByID(ctx context.Context, id uuid.UUID) (*model.NotificationLog, error) {
	var log model.NotificationLog
	if err := TenantDB(r.db, ctx).Preload("Template").Preload("Channel").Preload("ExecutionRun.Task").Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

// NotificationLogListOptions 通知日志列表查询选项
type NotificationLogListOptions struct {
	Page           int
	PageSize       int
	Status         string     // 状态筛选
	ChannelID      *uuid.UUID // 渠道 ID
	TemplateID     *uuid.UUID // 模板 ID
	TaskID         *uuid.UUID // 任务模板 ID
	TaskName       string     // 任务模板名称（模糊搜索）
	TriggeredBy    string     // 触发类型: manual, scheduler:cron, scheduler:once, healing
	ExecutionRunID *uuid.UUID // 执行记录 ID
	Search         string     // 模糊搜索主题
	CreatedAfter   *time.Time // 创建时间起始
	CreatedBefore  *time.Time // 创建时间结束
	SortBy         string     // 排序字段
	SortOrder      string     // 排序方向
}

// ListLogs 获取日志列表
func (r *NotificationRepository) ListLogs(ctx context.Context, opts *NotificationLogListOptions) ([]model.NotificationLog, int64, error) {
	var logs []model.NotificationLog
	var total int64

	tenantID := TenantIDFromContext(ctx)
	query := r.db.WithContext(ctx).Where("notification_logs.tenant_id = ?", tenantID).Model(&model.NotificationLog{})

	// 状态筛选
	if opts.Status != "" {
		query = query.Where("notification_logs.status = ?", opts.Status)
	}

	// 渠道筛选
	if opts.ChannelID != nil {
		query = query.Where("notification_logs.channel_id = ?", *opts.ChannelID)
	}

	// 模板筛选
	if opts.TemplateID != nil {
		query = query.Where("notification_logs.template_id = ?", *opts.TemplateID)
	}

	// 执行记录筛选
	if opts.ExecutionRunID != nil {
		query = query.Where("notification_logs.execution_run_id = ?", *opts.ExecutionRunID)
	}

	// 主题模糊搜索
	if opts.Search != "" {
		query = query.Where("notification_logs.subject ILIKE ?", "%"+opts.Search+"%")
	}

	// 时间范围筛选
	if opts.CreatedAfter != nil {
		query = query.Where("notification_logs.created_at >= ?", *opts.CreatedAfter)
	}
	if opts.CreatedBefore != nil {
		query = query.Where("notification_logs.created_at <= ?", *opts.CreatedBefore)
	}

	// 任务模板 ID / 名称 / 触发类型筛选（需要 JOIN execution_runs 和 execution_tasks）
	needJoin := opts.TaskID != nil || opts.TaskName != "" || opts.TriggeredBy != ""
	if needJoin {
		query = query.Joins("LEFT JOIN execution_runs ON notification_logs.execution_run_id = execution_runs.id")
		query = query.Joins("LEFT JOIN execution_tasks ON execution_runs.task_id = execution_tasks.id")

		if opts.TaskID != nil {
			query = query.Where("execution_runs.task_id = ?", *opts.TaskID)
		}
		if opts.TaskName != "" {
			query = query.Where("execution_tasks.name ILIKE ?", "%"+opts.TaskName+"%")
		}
		if opts.TriggeredBy != "" {
			query = query.Where("execution_runs.triggered_by = ?", opts.TriggeredBy)
		}
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	orderClause := "notification_logs.created_at DESC"
	if opts.SortBy != "" {
		order := "ASC"
		if opts.SortOrder == "desc" {
			order = "DESC"
		}
		// 白名单验证排序字段
		allowedSortFields := map[string]string{
			"created_at": "notification_logs.created_at",
			"status":     "notification_logs.status",
			"subject":    "notification_logs.subject",
			"sent_at":    "notification_logs.sent_at",
		}
		if field, ok := allowedSortFields[opts.SortBy]; ok {
			orderClause = field + " " + order
		}
	}

	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Select("notification_logs.*").Preload("Template").Preload("Channel").Preload("ExecutionRun.Task").Offset(offset).Limit(opts.PageSize).Order(orderClause).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// UpdateLog 更新日志
func (r *NotificationRepository) UpdateLog(ctx context.Context, log *model.NotificationLog) error {
	return r.db.WithContext(ctx).Save(log).Error
}

// GetPendingRetryLogs 获取待重试的日志
func (r *NotificationRepository) GetPendingRetryLogs(ctx context.Context) ([]model.NotificationLog, error) {
	var logs []model.NotificationLog
	if err := TenantDB(r.db, ctx).Preload("Channel").
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= NOW()", "failed").
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// ==================== 统计 ====================

// GetStats 获取通知统计信息
func (r *NotificationRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	// === 渠道统计 ===
	var channelsTotal int64
	if err := newDB().Model(&model.NotificationChannel{}).Count(&channelsTotal).Error; err != nil {
		return nil, err
	}
	stats["channels_total"] = channelsTotal

	// 渠道按类型统计
	type TypeCount struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var channelTypeCounts []TypeCount
	newDB().Model(&model.NotificationChannel{}).
		Select("type, count(*) as count").
		Group("type").
		Scan(&channelTypeCounts)
	stats["channels_by_type"] = channelTypeCounts

	// === 模板统计 ===
	var templatesTotal int64
	if err := newDB().Model(&model.NotificationTemplate{}).Count(&templatesTotal).Error; err != nil {
		return nil, err
	}
	stats["templates_total"] = templatesTotal

	var templatesActive int64
	newDB().Model(&model.NotificationTemplate{}).
		Where("is_active = ?", true).
		Count(&templatesActive)
	stats["templates_active"] = templatesActive

	// === 日志统计 ===
	var logsTotal int64
	if err := newDB().Model(&model.NotificationLog{}).Count(&logsTotal).Error; err != nil {
		return nil, err
	}
	stats["logs_total"] = logsTotal

	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var logStatusCounts []StatusCount
	newDB().Model(&model.NotificationLog{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&logStatusCounts)
	stats["logs_by_status"] = logStatusCounts

	return stats, nil
}
