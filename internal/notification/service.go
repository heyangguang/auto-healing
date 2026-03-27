package notification

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/company/auto-healing/internal/notification/provider"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service 通知服务
type Service struct {
	repo             *engagementrepo.NotificationRepository
	healingFlowRepo  *automationrepo.HealingFlowRepository
	providerRegistry *provider.Registry
	templateParser   *TemplateParser
	variableBuilder  *VariableBuilder
	rateLimiter      *RateLimiter
}

// NewService 创建通知服务
func NewService(db *gorm.DB, systemName, systemURL, systemVersion string) *Service {
	return &Service{
		repo:             engagementrepo.NewNotificationRepository(db),
		healingFlowRepo:  automationrepo.NewHealingFlowRepository(),
		providerRegistry: provider.NewRegistry(),
		templateParser:   NewTemplateParser(),
		variableBuilder:  NewVariableBuilder(systemName, systemURL, systemVersion),
		rateLimiter:      NewRateLimiter(),
	}
}

func NewConfiguredService(db *gorm.DB) *Service {
	appCfg := config.GetAppConfig()
	return NewService(db, appCfg.Name, appCfg.URL, appCfg.Version)
}

// CreateChannelRequest 创建渠道请求
type CreateChannelRequest struct {
	Name               string                 `json:"name" binding:"required"`
	Type               string                 `json:"type" binding:"required"`
	Description        string                 `json:"description"`
	Config             map[string]interface{} `json:"config" binding:"required"`
	RetryConfig        *model.RetryConfig     `json:"retry_config"`
	Recipients         []string               `json:"recipients"`
	IsDefault          bool                   `json:"is_default"`
	RateLimitPerMinute *int                   `json:"rate_limit_per_minute"`
}

// UpdateChannelRequest 更新渠道请求
type UpdateChannelRequest struct {
	Name               *string                `json:"name"`
	Description        *string                `json:"description"`
	Config             map[string]interface{} `json:"config"`
	RetryConfig        *model.RetryConfig     `json:"retry_config"`
	Recipients         []string               `json:"recipients"`
	IsActive           *bool                  `json:"is_active"`
	IsDefault          *bool                  `json:"is_default"`
	RateLimitPerMinute *int                   `json:"rate_limit_per_minute"`
}

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	Name              string   `json:"name" binding:"required"`
	Description       string   `json:"description"`
	EventType         string   `json:"event_type"`
	SupportedChannels []string `json:"supported_channels"`
	SubjectTemplate   string   `json:"subject_template"`
	BodyTemplate      string   `json:"body_template" binding:"required"`
	Format            string   `json:"format"`
	IsActive          *bool    `json:"is_active"`
}

// UpdateTemplateRequest 更新模板请求
type UpdateTemplateRequest struct {
	Name              *string  `json:"name"`
	Description       *string  `json:"description"`
	EventType         *string  `json:"event_type"`
	SupportedChannels []string `json:"supported_channels"`
	SubjectTemplate   *string  `json:"subject_template"`
	BodyTemplate      *string  `json:"body_template"`
	Format            *string  `json:"format"`
	IsActive          *bool    `json:"is_active"`
}

// PreviewResult 预览结果
type PreviewResult struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// SendNotificationRequest 发送通知请求
type SendNotificationRequest struct {
	TemplateID     *uuid.UUID             `json:"template_id"`
	ChannelIDs     []uuid.UUID            `json:"channel_ids"`
	Variables      map[string]interface{} `json:"variables"`
	Subject        string                 `json:"subject"`
	Body           string                 `json:"body"`
	Format         string                 `json:"format"`
	ExecutionRunID *uuid.UUID             `json:"execution_run_id"`
}

func (s *Service) GetChannel(ctx context.Context, id uuid.UUID) (*model.NotificationChannel, error) {
	channel, err := s.repo.GetChannelByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotificationChannelNotFound
	}
	return channel, err
}

func (s *Service) ListChannels(ctx context.Context, page, pageSize int, channelType string, name query.StringFilter) ([]model.NotificationChannel, int64, error) {
	return s.repo.ListChannels(ctx, page, pageSize, channelType, name)
}

func (s *Service) GetTemplate(ctx context.Context, id uuid.UUID) (*model.NotificationTemplate, error) {
	template, err := s.repo.GetTemplateByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotificationTemplateNotFound
	}
	return template, err
}

func (s *Service) ListTemplates(ctx context.Context, opts *engagementrepo.TemplateListOptions) ([]model.NotificationTemplate, int64, error) {
	return s.repo.ListTemplates(ctx, opts)
}

func (s *Service) GetAvailableVariables() []VariableInfo {
	return s.templateParser.GetAvailableVariables()
}

// GetNotification 获取通知日志
func (s *Service) GetNotification(ctx context.Context, id uuid.UUID) (*model.NotificationLog, error) {
	log, err := s.repo.GetLogByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotificationLogNotFound
	}
	return log, err
}

// ListNotifications 获取通知日志列表
func (s *Service) ListNotifications(ctx context.Context, opts *engagementrepo.NotificationLogListOptions) ([]model.NotificationLog, int64, error) {
	return s.repo.ListLogs(ctx, opts)
}
