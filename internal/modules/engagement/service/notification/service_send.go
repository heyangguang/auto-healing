package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/company/auto-healing/internal/modules/engagement/service/notification/provider"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
	sendErrs := make([]error, 0)
	for _, channel := range channels {
		log, sendErr := s.sendToChannel(ctx, &channel, subject, body, format, req.TemplateID, req.ExecutionRunID)
		if log != nil {
			logs = append(logs, log)
		}
		if sendErr != nil {
			sendErrs = append(sendErrs, fmt.Errorf("channel %s: %w", channel.ID, sendErr))
		}
	}
	if len(logs) > 0 && allNotificationLogsFailed(logs) {
		sendErrs = append(sendErrs, ErrNotificationSendAllFailed)
	}
	return logs, errors.Join(sendErrs...)
}

func (s *Service) resolveChannels(ctx context.Context, channelIDs []uuid.UUID) ([]model.NotificationChannel, error) {
	if len(channelIDs) == 0 {
		defaultChannel, err := s.repo.GetDefaultChannel(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("%w: 未指定渠道且没有可用的默认渠道", ErrNotificationChannelNotFound)
			}
			return nil, err
		}
		return []model.NotificationChannel{*defaultChannel}, nil
	}

	requestedIDs := uniqueNotificationChannelIDs(channelIDs)
	channels, err := s.repo.GetChannelsByIDs(ctx, requestedIDs)
	if err != nil {
		return nil, err
	}
	channelByID := make(map[uuid.UUID]model.NotificationChannel, len(channels))
	for _, channel := range channels {
		channelByID[channel.ID] = channel
	}
	resolved := make([]model.NotificationChannel, 0, len(requestedIDs))
	for _, channelID := range requestedIDs {
		channel, ok := channelByID[channelID]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrNotificationChannelNotFound, channelID)
		}
		if !channel.IsActive {
			return nil, fmt.Errorf("%w: %s", ErrNotificationChannelInactive, channelID)
		}
		resolved = append(resolved, channel)
	}
	return resolved, nil
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", "", fmt.Errorf("%w: %s", ErrNotificationTemplateNotFound, req.TemplateID.String())
		}
		return "", "", "", err
	}
	subject, _ = s.templateParser.Parse(template.SubjectTemplate, req.Variables)
	body, _ = s.templateParser.Parse(template.BodyTemplate, req.Variables)
	return subject, body, template.Format, nil
}

// sendToChannel 发送到单个渠道
func (s *Service) sendToChannel(ctx context.Context, channel *model.NotificationChannel, subject, body, format string, templateID, executionRunID *uuid.UUID) (*model.NotificationLog, error) {
	log := &model.NotificationLog{
		TemplateID:     templateID,
		ChannelID:      channel.ID,
		ExecutionRunID: executionRunID,
		Recipients:     channel.Recipients,
		Subject:        subject,
		Body:           body,
		ResponseData:   notificationRequestMetadata(format),
		Status:         "pending",
	}
	if err := s.repo.CreateLog(ctx, log); err != nil {
		return nil, fmt.Errorf("%w: create log: %v", ErrNotificationLogPersistenceFailed, err)
	}

	if err := s.checkSendRateLimit(ctx, channel, log); err != nil {
		return log, nil
	}

	providerImpl, ok := s.providerRegistry.Get(channel.Type)
	if !ok {
		log.Status = "failed"
		log.ErrorMessage = fmt.Sprintf("不支持的渠道类型: %s", channel.Type)
		if err := s.updateNotificationLog(ctx, log); err != nil {
			return log, err
		}
		return log, nil
	}

	resp, err := providerImpl.Send(ctx, &provider.SendRequest{
		Recipients: channel.Recipients,
		Subject:    subject,
		Body:       body,
		Format:     format,
		Config:     channel.Config,
	})
	if err := s.applySendResponse(ctx, channel, log, resp, err); err != nil {
		return log, err
	}
	return log, nil
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
	if err := s.updateNotificationLog(ctx, log); err != nil {
		return err
	}
	return fmt.Errorf("rate limited")
}

func (s *Service) applySendResponse(ctx context.Context, channel *model.NotificationChannel, log *model.NotificationLog, resp *provider.SendResponse, err error) error {
	now := time.Now()
	if err != nil || !resp.Success {
		log.Status = "failed"
		if err != nil {
			log.ErrorMessage = err.Error()
		} else {
			log.ErrorMessage = resp.ErrorMessage
		}
		s.scheduleNextRetry(channel, log, now)
		return s.updateNotificationLog(ctx, log)
	}

	log.Status = "sent"
	log.SentAt = &now
	log.ExternalMessageID = resp.ExternalMessageID
	if resp.ResponseData != nil {
		respJSON, marshalErr := json.Marshal(resp.ResponseData)
		if marshalErr != nil {
			return marshalErr
		}
		responseData := make(model.JSON, len(log.ResponseData))
		for key, value := range log.ResponseData {
			responseData[key] = value
		}
		if unmarshalErr := json.Unmarshal(respJSON, &responseData); unmarshalErr != nil {
			return unmarshalErr
		}
		log.ResponseData = responseData
	}
	return s.updateNotificationLog(ctx, log)
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

func (s *Service) updateNotificationLog(ctx context.Context, log *model.NotificationLog) error {
	if err := s.repo.UpdateLog(ctx, log); err != nil {
		return fmt.Errorf("%w: update log %s: %v", ErrNotificationLogPersistenceFailed, log.ID, err)
	}
	return nil
}

func uniqueNotificationChannelIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	return unique
}

func allNotificationLogsFailed(logs []*model.NotificationLog) bool {
	if len(logs) == 0 {
		return false
	}
	for _, log := range logs {
		if log == nil || log.Status == "sent" || log.Status == "delivered" {
			return false
		}
	}
	return true
}

func notificationRequestMetadata(format string) model.JSON {
	if format == "" {
		format = "text"
	}
	return model.JSON{"request_format": format}
}

func notificationRequestFormat(data model.JSON) string {
	if data == nil {
		return ""
	}
	if value, ok := data["request_format"].(string); ok {
		return value
	}
	return ""
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
