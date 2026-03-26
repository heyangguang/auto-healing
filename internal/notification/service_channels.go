package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

// CreateChannel 创建通知渠道
func (s *Service) CreateChannel(ctx context.Context, req CreateChannelRequest) (*model.NotificationChannel, error) {
	if _, ok := s.providerRegistry.Get(req.Type); !ok {
		return nil, fmt.Errorf("不支持的渠道类型: %s", req.Type)
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
	channel, err := s.repo.GetChannelByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
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

// DeleteChannel 删除渠道（保护性删除）
func (s *Service) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	channel, err := s.repo.GetChannelByID(ctx, id)
	if err != nil {
		return fmt.Errorf("渠道不存在: %w", err)
	}

	if err := s.ensureChannelNotInUse(ctx, id, channel.Type); err != nil {
		return err
	}
	return s.repo.DeleteChannel(ctx, id)
}

func (s *Service) ensureChannelNotInUse(ctx context.Context, channelID uuid.UUID, channelType string) error {
	templateCount, err := s.repo.CountTemplatesUsingChannelType(ctx, channelType)
	if err != nil {
		return fmt.Errorf("检查关联模板失败: %w", err)
	}
	if templateCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个通知模板支持此渠道类型（%s），请先修改这些模板的 supported_channels", templateCount, channelType)
	}

	taskCount, err := s.repo.CountTasksUsingChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("检查关联任务模板失败: %w", err)
	}
	if taskCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个任务模板使用此通知渠道，请先修改这些任务的通知配置", taskCount)
	}

	flowCount, err := s.healingFlowRepo.CountFlowsUsingChannel(ctx, channelID.String())
	if err != nil {
		return fmt.Errorf("检查关联自愈流程失败: %w", err)
	}
	if flowCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个自愈流程使用此通知渠道，请先修改这些流程的通知节点配置", flowCount)
	}
	return nil
}

// TestChannel 测试渠道
func (s *Service) TestChannel(ctx context.Context, id uuid.UUID) error {
	channel, err := s.repo.GetChannelByID(ctx, id)
	if err != nil {
		return err
	}

	providerImpl, ok := s.providerRegistry.Get(channel.Type)
	if !ok {
		return fmt.Errorf("不支持的渠道类型: %s", channel.Type)
	}

	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return providerImpl.Test(testCtx, channel.Config)
}
