package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/notification/provider"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service 通知服务
type Service struct {
	repo             *repository.NotificationRepository
	healingFlowRepo  *repository.HealingFlowRepository
	providerRegistry *provider.Registry
	templateParser   *TemplateParser
	variableBuilder  *VariableBuilder
	rateLimiter      *RateLimiter
}

// NewService 创建通知服务
func NewService(db *gorm.DB, systemName, systemURL, systemVersion string) *Service {
	return &Service{
		repo:             repository.NewNotificationRepository(db),
		healingFlowRepo:  repository.NewHealingFlowRepository(),
		providerRegistry: provider.NewRegistry(),
		templateParser:   NewTemplateParser(),
		variableBuilder:  NewVariableBuilder(systemName, systemURL, systemVersion),
		rateLimiter:      NewRateLimiter(),
	}
}

// ==================== 渠道管理 ====================

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

// CreateChannel 创建通知渠道
func (s *Service) CreateChannel(req CreateChannelRequest) (*model.NotificationChannel, error) {
	// 验证渠道类型
	if _, ok := s.providerRegistry.Get(req.Type); !ok {
		return nil, fmt.Errorf("不支持的渠道类型: %s", req.Type)
	}

	configJSON, _ := json.Marshal(req.Config)
	retryConfig := req.RetryConfig
	if retryConfig == nil {
		retryConfig = &model.RetryConfig{
			MaxRetries:     3,
			RetryIntervals: []int{1, 5, 15},
		}
	}

	channel := &model.NotificationChannel{
		Name:               req.Name,
		Type:               req.Type,
		Description:        req.Description,
		Config:             model.JSON{},
		RetryConfig:        retryConfig,
		Recipients:         req.Recipients,
		IsActive:           true,
		IsDefault:          req.IsDefault,
		RateLimitPerMinute: req.RateLimitPerMinute,
	}
	json.Unmarshal(configJSON, &channel.Config)

	if err := s.repo.CreateChannel(channel); err != nil {
		return nil, err
	}
	return channel, nil
}

// GetChannel 获取渠道
func (s *Service) GetChannel(id uuid.UUID) (*model.NotificationChannel, error) {
	return s.repo.GetChannelByID(id)
}

// ListChannels 获取渠道列表
func (s *Service) ListChannels(page, pageSize int, channelType string, search string) ([]model.NotificationChannel, int64, error) {
	return s.repo.ListChannels(page, pageSize, channelType, search)
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

// UpdateChannel 更新渠道
func (s *Service) UpdateChannel(id uuid.UUID, req UpdateChannelRequest) (*model.NotificationChannel, error) {
	channel, err := s.repo.GetChannelByID(id)
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
		configJSON, _ := json.Marshal(req.Config)
		json.Unmarshal(configJSON, &channel.Config)
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

	if err := s.repo.UpdateChannel(channel); err != nil {
		return nil, err
	}
	return channel, nil
}

// DeleteChannel 删除渠道（保护性删除）
func (s *Service) DeleteChannel(id uuid.UUID) error {
	// 获取渠道信息
	channel, err := s.repo.GetChannelByID(id)
	if err != nil {
		return fmt.Errorf("渠道不存在: %w", err)
	}

	// 检查是否被通知模板的 supported_channels 引用
	templateCount, err := s.repo.CountTemplatesUsingChannelType(channel.Type)
	if err != nil {
		return fmt.Errorf("检查关联模板失败: %w", err)
	}
	if templateCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个通知模板支持此渠道类型（%s），请先修改这些模板的 supported_channels", templateCount, channel.Type)
	}

	// 检查是否被任务模板的 notification_config.channel_ids 引用
	taskCount, err := s.repo.CountTasksUsingChannel(id)
	if err != nil {
		return fmt.Errorf("检查关联任务模板失败: %w", err)
	}
	if taskCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个任务模板使用此通知渠道，请先修改这些任务的通知配置", taskCount)
	}

	// 检查是否被自愈流程的 notification 节点引用
	flowCount, err := s.healingFlowRepo.CountFlowsUsingChannel(context.Background(), id.String())
	if err != nil {
		return fmt.Errorf("检查关联自愈流程失败: %w", err)
	}
	if flowCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个自愈流程使用此通知渠道，请先修改这些流程的通知节点配置", flowCount)
	}

	return s.repo.DeleteChannel(id)
}

// TestChannel 测试渠道
func (s *Service) TestChannel(id uuid.UUID) error {
	channel, err := s.repo.GetChannelByID(id)
	if err != nil {
		return err
	}

	p, ok := s.providerRegistry.Get(channel.Type)
	if !ok {
		return fmt.Errorf("不支持的渠道类型: %s", channel.Type)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return p.Test(ctx, channel.Config)
}

// ==================== 模板管理 ====================

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	Name              string   `json:"name" binding:"required"`
	Description       string   `json:"description"`
	EventType         string   `json:"event_type"`
	SupportedChannels []string `json:"supported_channels"`
	SubjectTemplate   string   `json:"subject_template"`
	BodyTemplate      string   `json:"body_template" binding:"required"`
	Format            string   `json:"format"` // text, markdown, html
}

// CreateTemplate 创建模板
func (s *Service) CreateTemplate(req CreateTemplateRequest) (*model.NotificationTemplate, error) {
	format := req.Format
	if format == "" {
		format = "text"
	}

	// 提取可用变量
	parser := NewTemplateParser()
	variables := parser.ExtractVariables(req.SubjectTemplate + " " + req.BodyTemplate)
	availableVars := make([]interface{}, len(variables))
	for i, v := range variables {
		availableVars[i] = v
	}

	template := &model.NotificationTemplate{
		Name:               req.Name,
		Description:        req.Description,
		EventType:          req.EventType,
		SupportedChannels:  req.SupportedChannels,
		SubjectTemplate:    req.SubjectTemplate,
		BodyTemplate:       req.BodyTemplate,
		Format:             format,
		AvailableVariables: model.JSONArray(availableVars),
		IsActive:           true,
	}

	if err := s.repo.CreateTemplate(template); err != nil {
		return nil, err
	}
	return template, nil
}

// GetTemplate 获取模板
func (s *Service) GetTemplate(id uuid.UUID) (*model.NotificationTemplate, error) {
	return s.repo.GetTemplateByID(id)
}

// ListTemplates 获取模板列表
func (s *Service) ListTemplates(opts *repository.TemplateListOptions) ([]model.NotificationTemplate, int64, error) {
	return s.repo.ListTemplates(opts)
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

// UpdateTemplate 更新模板
func (s *Service) UpdateTemplate(id uuid.UUID, req UpdateTemplateRequest) (*model.NotificationTemplate, error) {
	template, err := s.repo.GetTemplateByID(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		template.Name = *req.Name
	}
	if req.Description != nil {
		template.Description = *req.Description
	}
	if req.EventType != nil {
		template.EventType = *req.EventType
	}
	if req.SupportedChannels != nil {
		template.SupportedChannels = req.SupportedChannels
	}
	if req.SubjectTemplate != nil {
		template.SubjectTemplate = *req.SubjectTemplate
	}
	if req.BodyTemplate != nil {
		template.BodyTemplate = *req.BodyTemplate
	}
	if req.Format != nil {
		template.Format = *req.Format
	}
	if req.IsActive != nil {
		template.IsActive = *req.IsActive
	}

	// 重新提取变量
	parser := NewTemplateParser()
	variables := parser.ExtractVariables(template.SubjectTemplate + " " + template.BodyTemplate)
	availableVars := make([]interface{}, len(variables))
	for i, v := range variables {
		availableVars[i] = v
	}
	template.AvailableVariables = model.JSONArray(availableVars)
	template.UpdatedAt = time.Now()

	if err := s.repo.UpdateTemplate(template); err != nil {
		return nil, err
	}
	return template, nil
}

// DeleteTemplate 删除模板（保护性删除）
func (s *Service) DeleteTemplate(id uuid.UUID) error {
	// 检查是否被任务模板的 notification_config.template_id 引用
	taskCount, err := s.repo.CountTasksUsingTemplate(id)
	if err != nil {
		return fmt.Errorf("检查关联任务模板失败: %w", err)
	}
	if taskCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个任务模板使用此通知模板，请先修改这些任务的通知配置", taskCount)
	}

	// 检查是否被自愈流程的 notification 节点引用
	flowCount, err := s.healingFlowRepo.CountFlowsUsingTemplate(context.Background(), id.String())
	if err != nil {
		return fmt.Errorf("检查关联自愈流程失败: %w", err)
	}
	if flowCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个自愈流程使用此通知模板，请先修改这些流程的通知节点配置", flowCount)
	}

	return s.repo.DeleteTemplate(id)
}

// PreviewResult 预览结果
type PreviewResult struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// PreviewTemplate 预览模板
func (s *Service) PreviewTemplate(id uuid.UUID, variables map[string]interface{}) (*PreviewResult, error) {
	template, err := s.repo.GetTemplateByID(id)
	if err != nil {
		return nil, err
	}

	subject, err := s.templateParser.Parse(template.SubjectTemplate, variables)
	if err != nil {
		return nil, err
	}

	body, err := s.templateParser.Parse(template.BodyTemplate, variables)
	if err != nil {
		return nil, err
	}

	return &PreviewResult{Subject: subject, Body: body}, nil
}

// GetAvailableVariables 获取可用变量列表
func (s *Service) GetAvailableVariables() []VariableInfo {
	return s.templateParser.GetAvailableVariables()
}

// ==================== 通知发送 ====================

// SendNotificationRequest 发送通知请求
type SendNotificationRequest struct {
	TemplateID     *uuid.UUID             `json:"template_id"`
	ChannelIDs     []uuid.UUID            `json:"channel_ids"` // 可选，如果为空则使用默认渠道
	Variables      map[string]interface{} `json:"variables"`
	Subject        string                 `json:"subject"` // 直接指定主题（不使用模板时）
	Body           string                 `json:"body"`    // 直接指定内容（不使用模板时）
	Format         string                 `json:"format"`  // text, markdown, html
	ExecutionRunID *uuid.UUID             `json:"execution_run_id"`
}

// Send 发送通知
func (s *Service) Send(ctx context.Context, req SendNotificationRequest) ([]*model.NotificationLog, error) {
	var channels []model.NotificationChannel
	var err error

	// 如果没有指定渠道，使用默认渠道
	if len(req.ChannelIDs) == 0 {
		defaultChannel, err := s.repo.GetDefaultChannel()
		if err != nil {
			return nil, fmt.Errorf("未指定渠道且没有可用的默认渠道")
		}
		channels = []model.NotificationChannel{*defaultChannel}
	} else {
		channels, err = s.repo.GetChannelsByIDs(req.ChannelIDs)
		if err != nil {
			return nil, err
		}
		if len(channels) == 0 {
			return nil, fmt.Errorf("未找到指定的渠道")
		}
	}

	// 解析模板
	subject := req.Subject
	body := req.Body
	format := req.Format
	if format == "" {
		format = "text"
	}

	if req.TemplateID != nil {
		template, err := s.repo.GetTemplateByID(*req.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("模板不存在: %w", err)
		}

		subject, _ = s.templateParser.Parse(template.SubjectTemplate, req.Variables)
		body, _ = s.templateParser.Parse(template.BodyTemplate, req.Variables)
		format = template.Format
	}

	// 发送到每个渠道
	var logs []*model.NotificationLog
	for _, channel := range channels {
		log := s.sendToChannel(ctx, &channel, subject, body, format, req.TemplateID, req.ExecutionRunID)
		logs = append(logs, log)
	}

	return logs, nil
}

// sendToChannel 发送到单个渠道
func (s *Service) sendToChannel(ctx context.Context, channel *model.NotificationChannel, subject, body, format string, templateID, executionRunID *uuid.UUID) *model.NotificationLog {
	// 创建日志记录
	log := &model.NotificationLog{
		TemplateID:     templateID,
		ChannelID:      channel.ID,
		ExecutionRunID: executionRunID,
		Recipients:     channel.Recipients,
		Subject:        subject,
		Body:           body,
		Status:         "pending",
	}
	s.repo.CreateLog(log)

	// 速率限制检查
	if channel.RateLimitPerMinute != nil && *channel.RateLimitPerMinute > 0 {
		rateLimitKey := fmt.Sprintf("channel:%s", channel.ID.String())
		if !s.rateLimiter.Allow(rateLimitKey, *channel.RateLimitPerMinute, time.Minute) {
			log.Status = "failed"
			log.ErrorMessage = fmt.Sprintf("超出速率限制: %d 条/分钟", *channel.RateLimitPerMinute)
			s.repo.UpdateLog(log)
			return log
		}
	}

	// 获取提供者
	p, ok := s.providerRegistry.Get(channel.Type)
	if !ok {
		log.Status = "failed"
		log.ErrorMessage = fmt.Sprintf("不支持的渠道类型: %s", channel.Type)
		s.repo.UpdateLog(log)
		return log
	}

	// 发送
	sendReq := &provider.SendRequest{
		Recipients: channel.Recipients,
		Subject:    subject,
		Body:       body,
		Format:     format,
		Config:     channel.Config,
	}

	resp, err := p.Send(ctx, sendReq)
	now := time.Now()

	if err != nil || !resp.Success {
		log.Status = "failed"
		if err != nil {
			log.ErrorMessage = err.Error()
		} else {
			log.ErrorMessage = resp.ErrorMessage
		}
		// 设置重试时间
		if channel.RetryConfig != nil && log.RetryCount < channel.RetryConfig.MaxRetries {
			retryMinutes := 1
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
		if resp.ResponseData != nil {
			respJSON, _ := json.Marshal(resp.ResponseData)
			json.Unmarshal(respJSON, &log.ResponseData)
		}
	}

	s.repo.UpdateLog(log)
	return log
}

// SendFromExecution 从执行记录发送通知（根据状态获取对应配置）
func (s *Service) SendFromExecution(ctx context.Context, run *model.ExecutionRun, task *model.ExecutionTask) ([]*model.NotificationLog, error) {
	if task.NotificationConfig == nil || !task.NotificationConfig.Enabled {
		return nil, nil
	}

	// 根据执行状态获取对应的触发配置
	triggerConfig := task.NotificationConfig.GetTriggerConfig(run.Status)
	if triggerConfig == nil || !triggerConfig.Enabled {
		return nil, nil
	}

	// 构建变量
	variables := s.variableBuilder.BuildFromExecution(run, task)

	// 发送通知
	return s.Send(ctx, SendNotificationRequest{
		TemplateID:     triggerConfig.TemplateID,
		ChannelIDs:     triggerConfig.ChannelIDs,
		Variables:      variables,
		ExecutionRunID: &run.ID,
	})
}

// SendOnStart 发送开始执行通知
func (s *Service) SendOnStart(ctx context.Context, run *model.ExecutionRun, task *model.ExecutionTask) ([]*model.NotificationLog, error) {
	if task.NotificationConfig == nil || !task.NotificationConfig.Enabled {
		return nil, nil
	}

	// 获取开始时的触发配置
	triggerConfig := task.NotificationConfig.GetTriggerConfig("start")
	if triggerConfig == nil || !triggerConfig.Enabled {
		return nil, nil
	}

	// 构建变量
	variables := s.variableBuilder.BuildFromExecution(run, task)

	// 发送通知
	return s.Send(ctx, SendNotificationRequest{
		TemplateID:     triggerConfig.TemplateID,
		ChannelIDs:     triggerConfig.ChannelIDs,
		Variables:      variables,
		ExecutionRunID: &run.ID,
	})
}

// ==================== 通知日志 ====================

// GetNotification 获取通知日志
func (s *Service) GetNotification(id uuid.UUID) (*model.NotificationLog, error) {
	return s.repo.GetLogByID(id)
}

// ListNotifications 获取通知日志列表
func (s *Service) ListNotifications(opts *repository.NotificationLogListOptions) ([]model.NotificationLog, int64, error) {
	return s.repo.ListLogs(opts)
}

// RetryFailed 重试失败的通知
func (s *Service) RetryFailed(ctx context.Context) error {
	logs, err := s.repo.GetPendingRetryLogs()
	if err != nil {
		return err
	}

	for _, log := range logs {
		channel, err := s.repo.GetChannelByID(log.ChannelID)
		if err != nil {
			continue
		}

		log.RetryCount++
		log.NextRetryAt = nil

		p, ok := s.providerRegistry.Get(channel.Type)
		if !ok {
			log.ErrorMessage = fmt.Sprintf("不支持的渠道类型: %s", channel.Type)
			s.repo.UpdateLog(&log)
			continue
		}

		sendReq := &provider.SendRequest{
			Recipients: log.Recipients,
			Subject:    log.Subject,
			Body:       log.Body,
			Format:     "text",
			Config:     channel.Config,
		}

		resp, err := p.Send(ctx, sendReq)
		now := time.Now()

		if err != nil || !resp.Success {
			log.Status = "failed"
			if err != nil {
				log.ErrorMessage = err.Error()
			} else {
				log.ErrorMessage = resp.ErrorMessage
			}
			// 设置下次重试
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
		}

		s.repo.UpdateLog(&log)
	}

	return nil
}
