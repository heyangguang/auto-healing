package httpapi

import (
	"encoding/json"
	"testing"

	"github.com/company/auto-healing/internal/modules/automation/model"
)

func TestCreateFlowRequestSyncsAutoCloseFromEnabledClosePolicy(t *testing.T) {
	req := CreateFlowRequest{
		Name:                    "flow",
		AutoCloseSourceIncident: boolPtr(false),
		ClosePolicy:             json.RawMessage(`{"enabled":true,"solution_template_id":"11111111-1111-1111-1111-111111111111","trigger_on":"flow_success"}`),
	}

	flow := req.ToModel()

	if !flow.AutoCloseSourceIncident {
		t.Fatal("AutoCloseSourceIncident = false, want true when close_policy.enabled is true")
	}
}

func TestUpdateFlowRequestDisablesExistingClosePolicyWhenLegacyFlagIsTurnedOff(t *testing.T) {
	flow := &model.HealingFlow{
		AutoCloseSourceIncident: true,
		ClosePolicy: model.JSON{
			"enabled":              true,
			"solution_template_id": "11111111-1111-1111-1111-111111111111",
			"trigger_on":           "flow_success",
		},
	}
	req := UpdateFlowRequest{
		AutoCloseSourceIncident: boolPtr(false),
	}

	req.ApplyTo(flow)

	if flow.AutoCloseSourceIncident {
		t.Fatal("AutoCloseSourceIncident = true, want false")
	}
	if enabled, _ := flow.ClosePolicy["enabled"].(bool); enabled {
		t.Fatalf("ClosePolicy.enabled = true, want false; policy=%#v", flow.ClosePolicy)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
