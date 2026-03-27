package notification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/modules/engagement/service/notification/provider"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"gorm.io/gorm"
)

// RetryFailed 重试失败的通知
func (s *Service) RetryFailed(ctx context.Context) error {
	logs, err := s.repo.GetPendingRetryLogsGlobal(ctx)
	if err != nil {
		return err
	}

	retryErrs := make([]error, 0)
	for _, log := range logs {
		retryCtx := ctx
		if log.TenantID != nil {
			retryCtx = platformrepo.WithTenantID(ctx, *log.TenantID)
		}
		if err := s.retryNotificationLog(retryCtx, &log); err != nil {
			retryErrs = append(retryErrs, fmt.Errorf("retry log %s: %w", log.ID, err))
		}
	}
	return errors.Join(retryErrs...)
}

func (s *Service) retryNotificationLog(ctx context.Context, log *model.NotificationLog) error {
	channel, err := s.repo.GetChannelByID(ctx, log.ChannelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.ErrorMessage = ErrNotificationChannelNotFound.Error()
			log.NextRetryAt = nil
			return s.updateNotificationLog(ctx, log)
		}
		return err
	}
	if !channel.IsActive {
		log.Status = "failed"
		log.ErrorMessage = ErrNotificationChannelInactive.Error()
		log.NextRetryAt = nil
		return s.updateNotificationLog(ctx, log)
	}

	log.RetryCount++
	log.NextRetryAt = nil

	providerImpl, ok := s.providerRegistry.Get(channel.Type)
	if !ok {
		log.ErrorMessage = fmt.Sprintf("不支持的渠道类型: %s", channel.Type)
		return s.updateNotificationLog(ctx, log)
	}

	format, err := s.retryNotificationFormat(ctx, log)
	if err != nil {
		log.Status = "failed"
		log.ErrorMessage = err.Error()
		log.NextRetryAt = nil
		return s.updateNotificationLog(ctx, log)
	}

	resp, err := providerImpl.Send(ctx, &provider.SendRequest{
		Recipients: log.Recipients,
		Subject:    log.Subject,
		Body:       log.Body,
		Format:     format,
		Config:     channel.Config,
	})
	now := time.Now()
	if err != nil || !resp.Success {
		log.Status = "failed"
		if err != nil {
			log.ErrorMessage = err.Error()
		} else {
			log.ErrorMessage = resp.ErrorMessage
		}
		if channel.RetryConfig != nil && log.RetryCount < channel.RetryConfig.MaxRetries {
			retryMinutes := 15
			if len(channel.RetryConfig.RetryIntervals) > log.RetryCount {
				retryMinutes = channel.RetryConfig.RetryIntervals[log.RetryCount]
			}
			nextRetry := now.Add(time.Duration(retryMinutes) * time.Minute)
			log.NextRetryAt = &nextRetry
		}
	} else {
		log.Status = "sent"
		log.SentAt = &now
		log.ExternalMessageID = resp.ExternalMessageID
		log.ErrorMessage = ""
	}
	return s.updateNotificationLog(ctx, log)
}

func (s *Service) retryNotificationFormat(ctx context.Context, log *model.NotificationLog) (string, error) {
	if format := notificationRequestFormat(log.ResponseData); format != "" {
		return format, nil
	}
	if log.TemplateID == nil {
		return "text", nil
	}
	template, err := s.repo.GetTemplateByID(ctx, *log.TemplateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNotificationTemplateNotFound
		}
		return "", err
	}
	if template.Format == "" {
		return "text", nil
	}
	return template.Format, nil
}
