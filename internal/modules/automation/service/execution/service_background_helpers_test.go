package execution

import (
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
)

func TestToNotificationRunCopiesRuntimeFields(t *testing.T) {
	now := time.Now()
	run := &model.ExecutionRun{
		ID:                     uuid.New(),
		TaskID:                 uuid.New(),
		Status:                 "success",
		TriggeredBy:            "manual",
		StartedAt:              &now,
		RuntimeTargetHosts:     "192.168.31.100",
		RuntimeSecretsSourceIDs: model.StringArray{
			"source-a",
			"source-b",
		},
		RuntimeExtraVars: model.JSON{
			"fault_type": "service_down",
		},
		RuntimeSkipNotification: true,
	}

	notifyRun := toNotificationRun(run)
	if notifyRun == nil {
		t.Fatal("toNotificationRun() = nil")
	}
	if notifyRun.RuntimeTargetHosts != "192.168.31.100" {
		t.Fatalf("RuntimeTargetHosts = %q", notifyRun.RuntimeTargetHosts)
	}
	if len(notifyRun.RuntimeSecretsSourceIDs) != 2 {
		t.Fatalf("RuntimeSecretsSourceIDs len = %d", len(notifyRun.RuntimeSecretsSourceIDs))
	}
	if notifyRun.RuntimeExtraVars["fault_type"] != "service_down" {
		t.Fatalf("RuntimeExtraVars = %#v", notifyRun.RuntimeExtraVars)
	}
	if !notifyRun.RuntimeSkipNotification {
		t.Fatal("RuntimeSkipNotification = false, want true")
	}
}
