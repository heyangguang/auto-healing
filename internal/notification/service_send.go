package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/notification/provider"
	"github.com/google/uuid"
)

// Send 发送通知
func (s *Service) Send(ctx context.Context, req SendNotificationRequest) ([]*model.NotificationLog, error) {
	channels, err := s.resolveChannels(ctx, req.ChannelIDs)
	if err != nil {
		return nil, err
	}
	subject, body, format, err := s.resolveNotificationContent(ctx, req)
	if err != nil {
		return nil, err
	}

	logs := make([]*model.NotificationLog, 0, len(channels))
	for _, channel := range channels {
		logs = append(logs, s.sendToChannel(ctx, &channel, subject, body, format, req.TemplateID, req.ExecutionRunID))
	}
	return logs, nil
}

func (s *Service) resolveChannels(ctx context.Context, channelIDs []uuid.UUID) ([]model.NotificationChannel, error) {
	if len(channelIDs) == 0 {
		defaultChannel, err := s.repo.GetDefaultChannel(ctx)
		if err != nil {
			return nil, fmt.Errorf("未指定渠道且没有可用的默认渠道")
		}
		return []model.NotificationChannel{*defaultChannel}, nil
	}

	channels, err := s.repo.GetChannelsByIDs(ctx, channelIDs)
	if err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return nil, fmt.Errorf("未找到指定的渠道")
	}
	return channels, nil
}

func (s *Service) resolveNotificationContent(ctx context.Context, req SendNotificationRequest) (string, string, string, error) {
	subject := req.Subject
	body := req.Body
	format := req.Format
	if format == "" {
		format = "text"
	}
	if req.TemplateID == nil {
		return subject, body, format, nil
	}

	template, err := s.repo.GetTemplateByID(ctx, *req.TemplateID)
	if err != nil {
		return "", "", "", fmt.Errorf("模板不存在: %w", err)
	}
	subject, _ = s.templateParser.Parse(template.SubjectTemplate, req.Variables)
	body, _ = s.templateParser.Parse(template.BodyTemplate, req.Variables)
	return subject, body, template.Format, nil
}

// sendToChannel 发送到单个渠道
func (s *Service) sendToChannel(ctx context.Context, channel *model.NotificationChannel, subject, body, format string, templateID, executionRunID *uuid.UUID) *model.NotificationLog {
	log := &model.NotificationLog{
		TemplateID:     templateID,
		ChannelID:      channel.ID,
		ExecutionRunID: executionRunID,
		Recipients:     channel.Recipients,
		Subject:        subject,
		Body:           body,
		Status:         "pending",
	}
	s.repo.CreateLog(ctx, log)

	if err := s.checkSendRateLimit(ctx, channel, log); err != nil {
		return log
	}

	providerImpl, ok := s.providerRegistry.Get(channel.Type)
	if !ok {
		log.Status = "failed"
		log.ErrorMessage = fmt.Sprintf("不支持的渠道类型: %s", channel.Type)
		s.repo.UpdateLog(ctx, log)
		return log
	}

	resp, err := providerImpl.Send(ctx, &provider.SendRequest{
		Recipients: channel.Recipients,
		Subject:    subject,
		Body:       body,
		Format:     format,
		Config:     channel.Config,
	})
	s.applySendResponse(ctx, channel, log, resp, err)
	return log
}

func (s *Service) checkSendRateLimit(ctx context.Context, channel *model.NotificationChannel, log *model.NotificationLog) error {
	if channel.RateLimitPerMinute == nil || *channel.RateLimitPerMinute <= 0 {
		return nil
	}
	rateLimitKey := fmt.Sprintf("channel:%s", channel.ID.String())
	if s.rateLimiter.Allow(rateLimitKey, *channel.RateLimitPerMinute, time.Minute) {
		return nil
	}

	log.Status = "failed"
	log.ErrorMessage = fmt.Sprintf("超出速率限制: %d 条/分钟", *channel.RateLimitPerMinute)
	s.repo.UpdateLog(ctx, log)
	return fmt.Errorf("rate limited")
}

func (s *Service) applySendResponse(ctx context.Context, channel *model.NotificationChannel, log *model.NotificationLog, resp *provider.SendResponse, err error) {
	now := time.Now()
	if err != nil || !resp.Success {
		log.Status = "failed"
		if err != nil {
			log.ErrorMessage = err.Error()
		} else {
			log.ErrorMessage = resp.ErrorMessage
		}
		s.scheduleNextRetry(channel, log, now)
		s.repo.UpdateLog(ctx, log)
		return
	}

	log.Status = "sent"
	log.SentAt = &now
	log.ExternalMessageID = resp.ExternalMessageID
	if resp.ResponseData != nil {
		respJSON, _ := json.Marshal(resp.ResponseData)
		json.Unmarshal(respJSON, &log.ResponseData)
	}
	s.repo.UpdateLog(ctx, log)
}

func (s *Service) scheduleNextRetry(channel *model.NotificationChannel, log *model.NotificationLog, now time.Time) {
	if channel.RetryConfig == nil || log.RetryCount >= channel.RetryConfig.MaxRetries {
		return
	}
	retryMinutes := 1
	if len(channel.RetryConfig.RetryIntervals) > log.RetryCount {
		retryMinutes = channel.RetryConfig.RetryIntervals[log.RetryCount]
	}
	nextRetry := now.Add(time.Duration(retryMinutes) * time.Minute)
	log.NextRetryAt = &nextRetry
}

// SendFromExecution 从执行记录发送通知（根据状态获取对应配置）
func (s *Service) SendFromExecution(ctx context.Context, run *model.ExecutionRun, task *model.ExecutionTask) ([]*model.NotificationLog, error) {
	return s.sendFromExecutionTrigger(ctx, run, task, run.Status)
}

// SendOnStart 发送开始执行通知
func (s *Service) SendOnStart(ctx context.Context, run *model.ExecutionRun, task *model.ExecutionTask) ([]*model.NotificationLog, error) {
	return s.sendFromExecutionTrigger(ctx, run, task, "start")
}

func (s *Service) sendFromExecutionTrigger(ctx context.Context, run *model.ExecutionRun, task *model.ExecutionTask, trigger string) ([]*model.NotificationLog, error) {
	if task.NotificationConfig == nil || !task.NotificationConfig.Enabled {
		return nil, nil
	}
	triggerConfig := task.NotificationConfig.GetTriggerConfig(trigger)
	if triggerConfig == nil || !triggerConfig.Enabled {
		return nil, nil
	}

	return s.Send(ctx, SendNotificationRequest{
		TemplateID:     triggerConfig.TemplateID,
		ChannelIDs:     triggerConfig.ChannelIDs,
		Variables:      s.variableBuilder.BuildFromExecution(run, task),
		ExecutionRunID: &run.ID,
	})
}
