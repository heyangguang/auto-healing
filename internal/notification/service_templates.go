package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

// CreateTemplate 创建模板
func (s *Service) CreateTemplate(ctx context.Context, req CreateTemplateRequest) (*model.NotificationTemplate, error) {
	format := req.Format
	if format == "" {
		format = "text"
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	template := &model.NotificationTemplate{
		Name:               req.Name,
		Description:        req.Description,
		EventType:          req.EventType,
		SupportedChannels:  req.SupportedChannels,
		SubjectTemplate:    req.SubjectTemplate,
		BodyTemplate:       req.BodyTemplate,
		Format:             format,
		AvailableVariables: templateAvailableVariables(s.templateParser, req.SubjectTemplate, req.BodyTemplate),
		IsActive:           isActive,
	}
	if err := s.repo.CreateTemplate(ctx, template); err != nil {
		return nil, err
	}
	return template, nil
}

func templateAvailableVariables(parser *TemplateParser, subjectTemplate, bodyTemplate string) model.JSONArray {
	variables := parser.ExtractVariables(subjectTemplate + " " + bodyTemplate)
	availableVars := make([]interface{}, len(variables))
	for i, variable := range variables {
		availableVars[i] = variable
	}
	return model.JSONArray(availableVars)
}

// UpdateTemplate 更新模板
func (s *Service) UpdateTemplate(ctx context.Context, id uuid.UUID, req UpdateTemplateRequest) (*model.NotificationTemplate, error) {
	template, err := s.GetTemplate(ctx, id)
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

	template.AvailableVariables = templateAvailableVariables(s.templateParser, template.SubjectTemplate, template.BodyTemplate)
	template.UpdatedAt = time.Now()
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return nil, err
	}
	return template, nil
}

// DeleteTemplate 删除模板（保护性删除）
func (s *Service) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	taskCount, err := s.repo.CountTasksUsingTemplate(ctx, id)
	if err != nil {
		return fmt.Errorf("检查关联任务模板失败: %w", err)
	}
	if taskCount > 0 {
		return fmt.Errorf("%w: 无法删除：有 %d 个任务模板使用此通知模板，请先修改这些任务的通知配置", ErrNotificationResourceInUse, taskCount)
	}

	flowCount, err := s.healingFlowRepo.CountFlowsUsingTemplate(ctx, id.String())
	if err != nil {
		return fmt.Errorf("检查关联自愈流程失败: %w", err)
	}
	if flowCount > 0 {
		return fmt.Errorf("%w: 无法删除：有 %d 个自愈流程使用此通知模板，请先修改这些流程的通知节点配置", ErrNotificationResourceInUse, flowCount)
	}

	return s.repo.DeleteTemplate(ctx, id)
}

// PreviewTemplate 预览模板
func (s *Service) PreviewTemplate(ctx context.Context, id uuid.UUID, variables map[string]interface{}) (*PreviewResult, error) {
	template, err := s.GetTemplate(ctx, id)
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
