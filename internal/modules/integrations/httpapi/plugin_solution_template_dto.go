package httpapi

import integrationmodel "github.com/company/auto-healing/internal/modules/integrations/model"

type CreateSolutionTemplateRequest struct {
	Name               string `json:"name" binding:"required"`
	Description        string `json:"description"`
	ResolutionTemplate string `json:"resolution_template" binding:"required"`
	WorkNotesTemplate  string `json:"work_notes_template" binding:"required"`
	DefaultCloseCode   string `json:"default_close_code"`
	DefaultCloseStatus string `json:"default_close_status"`
}

func (r *CreateSolutionTemplateRequest) ToModel() *integrationmodel.IncidentSolutionTemplate {
	return &integrationmodel.IncidentSolutionTemplate{
		Name:               r.Name,
		Description:        r.Description,
		ResolutionTemplate: r.ResolutionTemplate,
		WorkNotesTemplate:  r.WorkNotesTemplate,
		DefaultCloseCode:   r.DefaultCloseCode,
		DefaultCloseStatus: r.DefaultCloseStatus,
	}
}

type UpdateSolutionTemplateRequest struct {
	Name               *string `json:"name"`
	Description        *string `json:"description"`
	ResolutionTemplate *string `json:"resolution_template"`
	WorkNotesTemplate  *string `json:"work_notes_template"`
	DefaultCloseCode   *string `json:"default_close_code"`
	DefaultCloseStatus *string `json:"default_close_status"`
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
	if r.DefaultCloseCode != nil {
		template.DefaultCloseCode = *r.DefaultCloseCode
	}
	if r.DefaultCloseStatus != nil {
		template.DefaultCloseStatus = *r.DefaultCloseStatus
	}
}
