package httpapi

import integrationmodel "github.com/company/auto-healing/internal/modules/integrations/model"

type CreateSolutionTemplateRequest struct {
	Name                 string `json:"name" binding:"required"`
	Description          string `json:"description"`
	ResolutionTemplate   string `json:"resolution_template"`
	WorkNotesTemplate    string `json:"work_notes_template"`
	ProblemTemplate      string `json:"problem_template"`
	SolutionTemplate     string `json:"solution_template"`
	VerificationTemplate string `json:"verification_template"`
	ConclusionTemplate   string `json:"conclusion_template"`
	StepsRenderMode      string `json:"steps_render_mode"`
	StepsMaxCount        int    `json:"steps_max_count"`
	StepOutputMaxLength  int    `json:"step_output_max_length"`
	DefaultCloseCode     string `json:"default_close_code"`
	DefaultCloseStatus   string `json:"default_close_status"`
}

func (r *CreateSolutionTemplateRequest) ToModel() *integrationmodel.IncidentSolutionTemplate {
	return &integrationmodel.IncidentSolutionTemplate{
		Name:                 r.Name,
		Description:          r.Description,
		ResolutionTemplate:   r.ResolutionTemplate,
		WorkNotesTemplate:    r.WorkNotesTemplate,
		ProblemTemplate:      r.ProblemTemplate,
		SolutionTemplate:     r.SolutionTemplate,
		VerificationTemplate: r.VerificationTemplate,
		ConclusionTemplate:   r.ConclusionTemplate,
		StepsRenderMode:      r.StepsRenderMode,
		StepsMaxCount:        r.StepsMaxCount,
		StepOutputMaxLength:  r.StepOutputMaxLength,
		DefaultCloseCode:     r.DefaultCloseCode,
		DefaultCloseStatus:   r.DefaultCloseStatus,
	}
}

type UpdateSolutionTemplateRequest struct {
	Name                 *string `json:"name"`
	Description          *string `json:"description"`
	ResolutionTemplate   *string `json:"resolution_template"`
	WorkNotesTemplate    *string `json:"work_notes_template"`
	ProblemTemplate      *string `json:"problem_template"`
	SolutionTemplate     *string `json:"solution_template"`
	VerificationTemplate *string `json:"verification_template"`
	ConclusionTemplate   *string `json:"conclusion_template"`
	StepsRenderMode      *string `json:"steps_render_mode"`
	StepsMaxCount        *int    `json:"steps_max_count"`
	StepOutputMaxLength  *int    `json:"step_output_max_length"`
	DefaultCloseCode     *string `json:"default_close_code"`
	DefaultCloseStatus   *string `json:"default_close_status"`
}

func (r *UpdateSolutionTemplateRequest) ApplyTo(template *integrationmodel.IncidentSolutionTemplate) {
	if r.Name != nil {
		template.Name = *r.Name
	}
	if r.Description != nil {
		template.Description = *r.Description
	}
	if r.ResolutionTemplate != nil {
		template.ResolutionTemplate = *r.ResolutionTemplate
	}
	if r.WorkNotesTemplate != nil {
		template.WorkNotesTemplate = *r.WorkNotesTemplate
	}
	if r.ProblemTemplate != nil {
		template.ProblemTemplate = *r.ProblemTemplate
	}
	if r.SolutionTemplate != nil {
		template.SolutionTemplate = *r.SolutionTemplate
	}
	if r.VerificationTemplate != nil {
		template.VerificationTemplate = *r.VerificationTemplate
	}
	if r.ConclusionTemplate != nil {
		template.ConclusionTemplate = *r.ConclusionTemplate
	}
	if r.StepsRenderMode != nil {
		template.StepsRenderMode = *r.StepsRenderMode
	}
	if r.StepsMaxCount != nil {
		template.StepsMaxCount = *r.StepsMaxCount
	}
	if r.StepOutputMaxLength != nil {
		template.StepOutputMaxLength = *r.StepOutputMaxLength
	}
	if r.DefaultCloseCode != nil {
		template.DefaultCloseCode = *r.DefaultCloseCode
	}
	if r.DefaultCloseStatus != nil {
		template.DefaultCloseStatus = *r.DefaultCloseStatus
	}
}
