package model

import (
	"time"

	"github.com/google/uuid"
)

// IncidentSolutionTemplate 工单关闭解决方案模板。
type IncidentSolutionTemplate struct {
	ID                   uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID             *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;uniqueIndex:idx_incident_solution_template_tenant_name"`
	Name                 string     `json:"name" gorm:"type:varchar(200);not null;uniqueIndex:idx_incident_solution_template_tenant_name"`
	Description          string     `json:"description,omitempty" gorm:"type:text"`
	ResolutionTemplate   string     `json:"resolution_template" gorm:"type:text;not null;default:''"`
	WorkNotesTemplate    string     `json:"work_notes_template" gorm:"type:text;not null;default:''"`
	ProblemTemplate      string     `json:"problem_template,omitempty" gorm:"type:text;not null;default:''"`
	SolutionTemplate     string     `json:"solution_template,omitempty" gorm:"type:text;not null;default:''"`
	VerificationTemplate string     `json:"verification_template,omitempty" gorm:"type:text;not null;default:''"`
	ConclusionTemplate   string     `json:"conclusion_template,omitempty" gorm:"type:text;not null;default:''"`
	StepsRenderMode      string     `json:"steps_render_mode,omitempty" gorm:"type:varchar(30);not null;default:'summary'"`
	StepsMaxCount        int        `json:"steps_max_count,omitempty" gorm:"not null;default:6"`
	StepOutputMaxLength  int        `json:"step_output_max_length,omitempty" gorm:"not null;default:240"`
	DefaultCloseCode     string     `json:"default_close_code,omitempty" gorm:"type:varchar(100)"`
	DefaultCloseStatus   string     `json:"default_close_status,omitempty" gorm:"type:varchar(50);default:'resolved'"`
	CreatedAt            time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt            time.Time  `json:"updated_at" gorm:"default:now()"`
}

func (IncidentSolutionTemplate) TableName() string {
	return "incident_solution_templates"
}

func (t IncidentSolutionTemplate) UsesStructuredSections() bool {
	return t.ProblemTemplate != "" ||
		t.SolutionTemplate != "" ||
		t.VerificationTemplate != "" ||
		t.ConclusionTemplate != ""
}
