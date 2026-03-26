package healing

import (
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestShouldFailExecuteError(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		started bool
		want    bool
	}{
		{name: "pending before start", status: model.FlowInstanceStatusPending, started: false, want: true},
		{name: "running before start", status: model.FlowInstanceStatusRunning, started: false, want: false},
		{name: "running after start", status: model.FlowInstanceStatusRunning, started: true, want: true},
		{name: "waiting approval after start", status: model.FlowInstanceStatusWaitingApproval, started: true, want: true},
		{name: "failed after start", status: model.FlowInstanceStatusFailed, started: true, want: false},
		{name: "cancelled after start", status: model.FlowInstanceStatusCancelled, started: true, want: false},
	}

	for _, tc := range cases {
		if got := shouldFailExecuteError(tc.status, tc.started); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
