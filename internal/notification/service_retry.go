package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/notification/provider"
	"github.com/company/auto-healing/internal/repository"
)

// RetryFailed 重试失败的通知
func (s *Service) RetryFailed(ctx context.Context) error {
	logs, err := s.repo.GetPendingRetryLogsGlobal(ctx)
	if err != nil {
		return err
	}

	for _, log := range logs {
		retryCtx := ctx
		if log.TenantID != nil {
			retryCtx = repository.WithTenantID(ctx, *log.TenantID)
		}
		if err := s.retryNotificationLog(retryCtx, &log); err != nil {
			continue
		}
	}
	return nil
}

func (s *Service) retryNotificationLog(ctx context.Context, log *model.NotificationLog) error {
	channel, err := s.repo.GetChannelByID(ctx, log.ChannelID)
	if err != nil {
		return err
	}

	log.RetryCount++
	log.NextRetryAt = nil

	providerImpl, ok := s.providerRegistry.Get(channel.Type)
	if !ok {
		log.ErrorMessage = fmt.Sprintf("不支持的渠道类型: %s", channel.Type)
		s.repo.UpdateLog(ctx, log)
		return fmt.Errorf("unsupported channel")
	}

	resp, err := providerImpl.Send(ctx, &provider.SendRequest{
		Recipients: log.Recipients,
		Subject:    log.Subject,
		Body:       log.Body,
		Format:     "text",
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
	return s.repo.UpdateLog(ctx, log)
}
