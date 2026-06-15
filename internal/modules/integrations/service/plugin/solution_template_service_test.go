package plugin

import (
	"testing"

	integrationmodel "github.com/company/auto-healing/internal/modules/integrations/model"
)

func TestValidateSolutionTemplateRejectsBlankName(t *testing.T) {
	template := &integrationmodel.IncidentSolutionTemplate{
		Name:               "   ",
		SolutionTemplate:   "方案",
		ConclusionTemplate: "结论",
	}

	if err := validateSolutionTemplate(template); err == nil {
		t.Fatal("expected blank solution template name to be rejected")
	}
}

func TestValidateSolutionTemplateTrimsFields(t *testing.T) {
	template := &integrationmodel.IncidentSolutionTemplate{
		Name:               "  模板  ",
		Description:        "  描述  ",
		SolutionTemplate:   "  方案  ",
		ConclusionTemplate: "  结论  ",
	}

	if err := validateSolutionTemplate(template); err != nil {
		t.Fatalf("validateSolutionTemplate() error = %v", err)
	}
	if template.Name != "模板" || template.Description != "描述" || template.SolutionTemplate != "方案" || template.ConclusionTemplate != "结论" {
		t.Fatalf("template fields were not trimmed: %+v", template)
	}
}
