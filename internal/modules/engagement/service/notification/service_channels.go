package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateChannel 创建通知渠道
func (s *Service) CreateChannel(ctx context.Context, req CreateChannelRequest) (*model.NotificationChannel, error) {
	if _, ok := s.providerRegistry.Get(req.Type); !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotificationUnsupportedType, req.Type)
	}
	if err := s.ensureChannelNameAvailable(ctx, nil, req.Name); err != nil {
		return nil, err
	}

	channel := &model.NotificationChannel{
		Name:               req.Name,
		Type:               req.Type,
		Description:        req.Description,
		Config:             model.JSON{},
		RetryConfig:        notificationRetryConfig(req.RetryConfig),
		Recipients:         req.Recipients,
		IsActive:           true,
		IsDefault:          req.IsDefault,
		RateLimitPerMinute: req.RateLimitPerMinute,
	}
	applyChannelConfig(channel, req.Config)

	if err := s.repo.CreateChannel(ctx, channel); err != nil {
		if isNotificationChannelNameConflict(err) {
			return nil, ErrNotificationChannelExists
		}
		return nil, err
	}
	return channel, nil
}

func notificationRetryConfig(retryConfig *model.RetryConfig) *model.RetryConfig {
	if retryConfig != nil {
		return retryConfig
	}
	return &model.RetryConfig{MaxRetries: 3, RetryIntervals: []int{1, 5, 15}}
}

func applyChannelConfig(channel *model.NotificationChannel, config map[string]interface{}) {
	configJSON, _ := json.Marshal(config)
	json.Unmarshal(configJSON, &channel.Config)
}

// UpdateChannel 更新渠道
func (s *Service) UpdateChannel(ctx context.Context, id uuid.UUID, req UpdateChannelRequest) (*model.NotificationChannel, error) {
	channel, err := s.GetChannel(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if err := s.ensureChannelNameAvailable(ctx, &channel.ID, *req.Name); err != nil {
			return nil, err
		}
		channel.Name = *req.Name
	}
	if req.Description != nil {
		channel.Description = *req.Description
	}
	if req.Config != nil {
		channel.Config = mergeChannelConfig(channel.Config, req.Config)
	}
	if req.RetryConfig != nil {
		channel.RetryConfig = req.RetryConfig
	}
	if req.Recipients != nil {
		channel.Recipients = req.Recipients
	}
	if req.IsActive != nil {
		channel.IsActive = *req.IsActive
	}
	if req.IsDefault != nil {
		channel.IsDefault = *req.IsDefault
	}
	if req.RateLimitPerMinute != nil {
		channel.RateLimitPerMinute = req.RateLimitPerMinute
	}
	channel.UpdatedAt = time.Now()

	if err := s.repo.UpdateChannel(ctx, channel); err != nil {
		if isNotificationChannelNameConflict(err) {
			return nil, ErrNotificationChannelExists
		}
		return nil, err
	}
	return channel, nil
}

func mergeChannelConfig(current model.JSON, updates map[string]interface{}) model.JSON {
	merged := map[string]interface{}{}
	if current != nil {
		for key, value := range current {
			merged[key] = value
		}
	}
	for key, value := range updates {
		merged[key] = value
	}
	configJSON, _ := json.Marshal(merged)
	var result model.JSON
	json.Unmarshal(configJSON, &result)
	return result
}

func (s *Service) ensureChannelNameAvailable(ctx context.Context, currentID *uuid.UUID, name string) error {
	existing, err := s.repo.GetChannelByName(ctx, name)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if currentID != nil && existing.ID == *currentID {
		return nil
	}
	return ErrNotificationChannelExists
}

func isNotificationChannelNameConflict(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "idx_channel_tenant_name"):
		return true
	case strings.Contains(lower, "duplicate key value") && strings.Contains(lower, "notification_channels"):
		return true
	case strings.Contains(lower, "unique constraint failed") && strings.Contains(lower, "notification_channels"):
		return true
	default:
		return false
	}
}

// DeleteChannel 删除渠道（保护性删除）
func (s *Service) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	channel, err := s.GetChannel(ctx, id)
	if err != nil {
		return err
	}

	if err := s.ensureChannelNotInUse(ctx, id, channel.Type); err != nil {
		return err
	}
	return s.repo.DeleteChannel(ctx, id)
}

func (s *Service) ensureChannelNotInUse(ctx context.Context, channelID uuid.UUID, channelType string) error {
	taskCount, err := s.repo.CountTasksUsingChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("检查关联任务模板失败: %w", err)
	}
	if taskCount > 0 {
		return fmt.Errorf("%w: 无法删除：有 %d 个任务模板使用此通知渠道，请先修改这些任务的通知配置", ErrNotificationResourceInUse, taskCount)
	}

	flowCount, err := s.healingFlowRepo.CountFlowsUsingChannel(ctx, channelID.String())
	if err != nil {
		return fmt.Errorf("检查关联自愈流程失败: %w", err)
	}
	if flowCount > 0 {
		return fmt.Errorf("%w: 无法删除：有 %d 个自愈流程使用此通知渠道，请先修改这些流程的通知节点配置", ErrNotificationResourceInUse, flowCount)
	}
	return nil
}

// TestChannel 测试渠道
func (s *Service) TestChannel(ctx context.Context, id uuid.UUID) error {
	channel, err := s.GetChannel(ctx, id)
	if err != nil {
		return err
	}

	providerImpl, ok := s.providerRegistry.Get(channel.Type)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotificationUnsupportedType, channel.Type)
	}

	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return providerImpl.Test(testCtx, channel.Config)
}
