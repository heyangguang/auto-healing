package repository

import (
	"context"
	"fmt"

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
	if channel.ID == uuid.Nil {
		channel.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if channel.IsDefault {
			if err := lockNotificationChannelDefaults(tx); err != nil {
				return err
			}
			if err := clearNotificationChannelDefaults(tx, channel.TenantID, uuid.Nil); err != nil {
				return err
			}
		}
		return tx.Create(channel).Error
	})
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if channel.IsDefault {
			if err := lockNotificationChannelDefaults(tx); err != nil {
				return err
			}
			if err := clearNotificationChannelDefaults(tx, channel.TenantID, channel.ID); err != nil {
				return err
			}
		}
		return UpdateTenantScopedModel(tx, ctx, channel.ID, channel)
	})
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

// CountTasksUsingTemplate 统计使用指定模板的任务模板数量
func (r *NotificationRepository) CountTasksUsingTemplate(ctx context.Context, templateID uuid.UUID) (int64, error) {
	var count int64
	query := r.tenantDB(ctx).Model(&model.ExecutionTask{})
	if r.db.Dialector.Name() == "sqlite" {
		err := query.Where(`(
			json_extract(notification_config, '$.on_start.template_id') = ? OR
			json_extract(notification_config, '$.on_success.template_id') = ? OR
			json_extract(notification_config, '$.on_failure.template_id') = ?)`,
			templateID.String(), templateID.String(), templateID.String()).
			Count(&count).Error
		return count, err
	}
	err := query.Where(`(
		notification_config->'on_start'->>'template_id' = ? OR
		notification_config->'on_success'->>'template_id' = ? OR
		notification_config->'on_failure'->>'template_id' = ?)`,
		templateID.String(), templateID.String(), templateID.String()).
		Count(&count).Error
	return count, err
}

// CountTasksUsingChannel 统计使用指定渠道的任务模板数量
func (r *NotificationRepository) CountTasksUsingChannel(ctx context.Context, channelID uuid.UUID) (int64, error) {
	var count int64
	query := r.tenantDB(ctx).Model(&model.ExecutionTask{})
	if r.db.Dialector.Name() == "sqlite" {
		err := query.Where(`(
			EXISTS (SELECT 1 FROM json_each(COALESCE(json_extract(notification_config, '$.on_start.channel_ids'), '[]')) WHERE value = ?) OR
			EXISTS (SELECT 1 FROM json_each(COALESCE(json_extract(notification_config, '$.on_success.channel_ids'), '[]')) WHERE value = ?) OR
			EXISTS (SELECT 1 FROM json_each(COALESCE(json_extract(notification_config, '$.on_failure.channel_ids'), '[]')) WHERE value = ?))`,
			channelID.String(), channelID.String(), channelID.String()).
			Count(&count).Error
		return count, err
	}
	channelIDsJSON := `["` + channelID.String() + `"]`
	err := query.Where(`(
		COALESCE(notification_config->'on_start'->'channel_ids', '[]'::jsonb) @> ?::jsonb OR
		COALESCE(notification_config->'on_success'->'channel_ids', '[]'::jsonb) @> ?::jsonb OR
		COALESCE(notification_config->'on_failure'->'channel_ids', '[]'::jsonb) @> ?::jsonb)`,
		channelIDsJSON, channelIDsJSON, channelIDsJSON).
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

func clearNotificationChannelDefaults(tx *gorm.DB, tenantID *uuid.UUID, excludeID uuid.UUID) error {
	query := tx.Model(&model.NotificationChannel{})
	if tenantID == nil {
		query = query.Where("tenant_id IS NULL")
	} else {
		query = query.Where("tenant_id = ?", *tenantID)
	}
	if excludeID != uuid.Nil {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
		return fmt.Errorf("clear default channels: %w", err)
	}
	return nil
}

func lockNotificationChannelDefaults(tx *gorm.DB) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	return tx.Exec("LOCK TABLE notification_channels IN SHARE ROW EXCLUSIVE MODE").Error
}
