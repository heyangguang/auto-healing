package repository

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateChannel 创建通知渠道
func (r *NotificationRepository) CreateChannel(ctx context.Context, channel *model.NotificationChannel) error {
	if err := FillTenantID(ctx, &channel.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(channel).Error
}

// GetChannelByID 根据 ID 获取渠道
func (r *NotificationRepository) GetChannelByID(ctx context.Context, id uuid.UUID) (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := r.tenantDB(ctx).Where("id = ?", id).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetChannelByName 根据名称获取渠道
func (r *NotificationRepository) GetChannelByName(ctx context.Context, name string) (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := r.tenantDB(ctx).Where("name = ?", name).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// ListChannels 获取渠道列表
func (r *NotificationRepository) ListChannels(ctx context.Context, page, pageSize int, channelType string, name query.StringFilter) ([]model.NotificationChannel, int64, error) {
	var channels []model.NotificationChannel
	queryBuilder := r.applyChannelFilters(r.tenantDB(ctx).Model(&model.NotificationChannel{}), channelType, name)
	total, err := countWithSession(queryBuilder)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err = queryBuilder.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&channels).Error
	if err != nil {
		return nil, 0, err
	}
	return channels, total, nil
}

// UpdateChannel 更新渠道
func (r *NotificationRepository) UpdateChannel(ctx context.Context, channel *model.NotificationChannel) error {
	return UpdateTenantScopedModel(r.db, ctx, channel.ID, channel)
}

// DeleteChannel 删除渠道
func (r *NotificationRepository) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	return r.tenantDB(ctx).Delete(&model.NotificationChannel{}, "id = ?", id).Error
}

// GetDefaultChannel 获取默认渠道
func (r *NotificationRepository) GetDefaultChannel(ctx context.Context) (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := r.tenantDB(ctx).Where("is_default = ? AND is_active = ?", true, true).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetChannelsByIDs 批量获取渠道
func (r *NotificationRepository) GetChannelsByIDs(ctx context.Context, ids []uuid.UUID) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	if err := r.tenantDB(ctx).Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// CountTemplatesUsingChannelType 统计使用指定渠道类型的模板数量
func (r *NotificationRepository) CountTemplatesUsingChannelType(ctx context.Context, channelType string) (int64, error) {
	var count int64
	err := r.tenantDB(ctx).Model(&model.NotificationTemplate{}).
		Where("supported_channels @> ?", `["`+channelType+`"]`).
		Count(&count).Error
	return count, err
}

// CountTasksUsingTemplate 统计使用指定模板的任务模板数量
func (r *NotificationRepository) CountTasksUsingTemplate(ctx context.Context, templateID uuid.UUID) (int64, error) {
	var count int64
	err := r.tenantDB(ctx).Model(&model.ExecutionTask{}).
		Where("notification_config->>'template_id' = ?", templateID.String()).
		Count(&count).Error
	return count, err
}

// CountTasksUsingChannel 统计使用指定渠道的任务模板数量
func (r *NotificationRepository) CountTasksUsingChannel(ctx context.Context, channelID uuid.UUID) (int64, error) {
	var count int64
	err := r.tenantDB(ctx).Model(&model.ExecutionTask{}).
		Where("notification_config->'channel_ids' @> ?", `["`+channelID.String()+`"]`).
		Count(&count).Error
	return count, err
}

func (r *NotificationRepository) applyChannelFilters(db *gorm.DB, channelType string, name query.StringFilter) *gorm.DB {
	if channelType != "" {
		db = db.Where("type = ?", channelType)
	}
	if name.IsEmpty() {
		return db
	}
	if name.Exact {
		return db.Where("name = ?", name.Value)
	}
	pattern := "%" + name.Value + "%"
	return db.Where("(name ILIKE ? OR description ILIKE ?)", pattern, pattern)
}
