package repository

import (
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
func (r *NotificationRepository) CreateChannel(channel *model.NotificationChannel) error {
	return r.db.Create(channel).Error
}

// GetChannelByID 根据 ID 获取渠道
func (r *NotificationRepository) GetChannelByID(id uuid.UUID) (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := r.db.Where("id = ?", id).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetChannelByName 根据名称获取渠道
func (r *NotificationRepository) GetChannelByName(name string) (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := r.db.Where("name = ?", name).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// ListChannels 获取渠道列表
func (r *NotificationRepository) ListChannels(page, pageSize int, channelType string) ([]model.NotificationChannel, int64, error) {
	var channels []model.NotificationChannel
	var total int64

	query := r.db.Model(&model.NotificationChannel{})
	if channelType != "" {
		query = query.Where("type = ?", channelType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, 0, err
	}

	return channels, total, nil
}

// UpdateChannel 更新渠道
func (r *NotificationRepository) UpdateChannel(channel *model.NotificationChannel) error {
	return r.db.Save(channel).Error
}

// DeleteChannel 删除渠道
func (r *NotificationRepository) DeleteChannel(id uuid.UUID) error {
	return r.db.Delete(&model.NotificationChannel{}, "id = ?", id).Error
}

// GetDefaultChannel 获取默认渠道
func (r *NotificationRepository) GetDefaultChannel() (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := r.db.Where("is_default = ? AND is_active = ?", true, true).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetChannelsByIDs 批量获取渠道
func (r *NotificationRepository) GetChannelsByIDs(ids []uuid.UUID) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	if err := r.db.Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// CountTemplatesUsingChannelType 统计使用指定渠道类型的模板数量
func (r *NotificationRepository) CountTemplatesUsingChannelType(channelType string) (int64, error) {
	var count int64
	// 检查 supported_channels 数组是否包含该渠道类型
	err := r.db.Model(&model.NotificationTemplate{}).
		Where("supported_channels @> ?", `["`+channelType+`"]`).
		Count(&count).Error
	return count, err
}

// CountTasksUsingTemplate 统计使用指定模板的任务模板数量
func (r *NotificationRepository) CountTasksUsingTemplate(templateID uuid.UUID) (int64, error) {
	var count int64
	// 检查 notification_config.template_id 是否等于该模板 ID
	err := r.db.Model(&model.ExecutionTask{}).
		Where("notification_config->>'template_id' = ?", templateID.String()).
		Count(&count).Error
	return count, err
}

// CountTasksUsingChannel 统计使用指定渠道的任务模板数量
func (r *NotificationRepository) CountTasksUsingChannel(channelID uuid.UUID) (int64, error) {
	var count int64
	// 检查 notification_config.channel_ids 数组是否包含该渠道 ID
	err := r.db.Model(&model.ExecutionTask{}).
		Where("notification_config->'channel_ids' @> ?", `["`+channelID.String()+`"]`).
		Count(&count).Error
	return count, err
}

// ==================== 模板管理 ====================

// CreateTemplate 创建通知模板
func (r *NotificationRepository) CreateTemplate(template *model.NotificationTemplate) error {
	return r.db.Create(template).Error
}

// GetTemplateByID 根据 ID 获取模板
func (r *NotificationRepository) GetTemplateByID(id uuid.UUID) (*model.NotificationTemplate, error) {
	var template model.NotificationTemplate
	if err := r.db.Where("id = ?", id).First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
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
func (r *NotificationRepository) ListTemplates(opts *TemplateListOptions) ([]model.NotificationTemplate, int64, error) {
	var templates []model.NotificationTemplate
	var total int64

	query := r.db.Model(&model.NotificationTemplate{})

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

	if err := query.Count(&total).Error; err != nil {
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
func (r *NotificationRepository) UpdateTemplate(template *model.NotificationTemplate) error {
	return r.db.Save(template).Error
}

// DeleteTemplate 删除模板
func (r *NotificationRepository) DeleteTemplate(id uuid.UUID) error {
	return r.db.Delete(&model.NotificationTemplate{}, "id = ?", id).Error
}

// ==================== 通知日志管理 ====================

// CreateLog 创建通知日志
func (r *NotificationRepository) CreateLog(log *model.NotificationLog) error {
	return r.db.Create(log).Error
}

// GetLogByID 根据 ID 获取日志
func (r *NotificationRepository) GetLogByID(id uuid.UUID) (*model.NotificationLog, error) {
	var log model.NotificationLog
	if err := r.db.Preload("Template").Preload("Channel").Preload("ExecutionRun.Task").Where("id = ?", id).First(&log).Error; err != nil {
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
func (r *NotificationRepository) ListLogs(opts *NotificationLogListOptions) ([]model.NotificationLog, int64, error) {
	var logs []model.NotificationLog
	var total int64

	query := r.db.Model(&model.NotificationLog{})

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

	if err := query.Count(&total).Error; err != nil {
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
func (r *NotificationRepository) UpdateLog(log *model.NotificationLog) error {
	return r.db.Save(log).Error
}

// GetPendingRetryLogs 获取待重试的日志
func (r *NotificationRepository) GetPendingRetryLogs() ([]model.NotificationLog, error) {
	var logs []model.NotificationLog
	if err := r.db.Preload("Channel").
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= NOW()", "failed").
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
