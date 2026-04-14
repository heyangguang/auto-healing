package healing

import (
	"testing"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
)

func TestResolveFlowContextSourceValueSupportsIncidentPrefix(t *testing.T) {
	flowContext := map[string]interface{}{
		"incident": map[string]interface{}{
			"affected_ci": "real-host-100",
		},
	}

	got := resolveFlowContextSourceValue(flowContext, "incident.affected_ci")
	if got != "real-host-100" {
		t.Fatalf("got %v, want real-host-100", got)
	}
}

func TestResolveFlowContextSourceValueSupportsPlainIncidentField(t *testing.T) {
	flowContext := map[string]interface{}{
		"incident": &platformmodel.Incident{
			AffectedCI: "real-host-101",
		},
	}

	got := resolveFlowContextSourceValue(flowContext, "affected_ci")
	if got != "real-host-101" {
		t.Fatalf("got %v, want real-host-101", got)
	}
}
