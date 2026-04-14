package plugin

import (
	"context"
	"fmt"
	"strings"

	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	integrationmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/google/uuid"
)

type SolutionTemplateService struct {
	repo     *integrationrepo.IncidentSolutionTemplateRepository
	flowRepo *automationrepo.HealingFlowRepository
}

type SolutionTemplateServiceDeps struct {
	Repo     *integrationrepo.IncidentSolutionTemplateRepository
	FlowRepo *automationrepo.HealingFlowRepository
}

func NewSolutionTemplateServiceWithDeps(deps SolutionTemplateServiceDeps) *SolutionTemplateService {
	switch {
	case deps.Repo == nil:
		panic("integrations solution template service requires repo")
	case deps.FlowRepo == nil:
		panic("integrations solution template service requires healing flow repo")
	}
	return &SolutionTemplateService{
		repo:     deps.Repo,
		flowRepo: deps.FlowRepo,
	}
}

func (s *SolutionTemplateService) List(ctx context.Context) ([]integrationmodel.IncidentSolutionTemplate, error) {
	return s.repo.List(ctx)
}

func (s *SolutionTemplateService) Get(ctx context.Context, id uuid.UUID) (*integrationmodel.IncidentSolutionTemplate, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SolutionTemplateService) Create(ctx context.Context, template *integrationmodel.IncidentSolutionTemplate) error {
	if err := validateSolutionTemplate(template); err != nil {
		return err
	}
	return s.repo.Create(ctx, template)
}

func (s *SolutionTemplateService) Update(ctx context.Context, template *integrationmodel.IncidentSolutionTemplate) error {
	if err := validateSolutionTemplate(template); err != nil {
		return err
	}
	return s.repo.Update(ctx, template)
}

func (s *SolutionTemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	count, err := s.flowRepo.CountFlowsUsingCloseTemplate(ctx, id.String())
	if err != nil {
		return fmt.Errorf("检查流程引用关系失败: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("无法删除：有 %d 个自愈流程正在使用该解决方案模板", count)
	}
	return s.repo.Delete(ctx, id)
}

func validateSolutionTemplate(template *integrationmodel.IncidentSolutionTemplate) error {
	if template == nil {
		return fmt.Errorf("解决方案模板不能为空")
	}
	template.Name = strings.TrimSpace(template.Name)
	template.Description = strings.TrimSpace(template.Description)
	template.ResolutionTemplate = strings.TrimSpace(template.ResolutionTemplate)
	template.WorkNotesTemplate = strings.TrimSpace(template.WorkNotesTemplate)
	template.DefaultCloseCode = strings.TrimSpace(template.DefaultCloseCode)
	template.DefaultCloseStatus = strings.TrimSpace(template.DefaultCloseStatus)
	if template.Name == "" {
		return fmt.Errorf("模板名称不能为空")
	}
	if template.ResolutionTemplate == "" {
		return fmt.Errorf("resolution_template 不能为空")
	}
	if template.WorkNotesTemplate == "" {
		return fmt.Errorf("work_notes_template 不能为空")
	}
	if template.DefaultCloseStatus == "" {
		template.DefaultCloseStatus = "resolved"
	}
	return nil
}
