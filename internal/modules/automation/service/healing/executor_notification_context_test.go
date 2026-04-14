package healing

import (
	"testing"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
)

func TestBuildNotificationVariablesUsesExecutionOutcomeDetails(t *testing.T) {
	instanceID := uuid.New()
	executor := &FlowExecutor{}
	instance := &model.FlowInstance{
		ID:     instanceID,
		Status: model.FlowInstanceStatusRunning,
		Context: model.JSON{
			"incident": map[string]interface{}{
				"affected_ci": "real-host-77",
			},
			"execution_result": map[string]interface{}{
				"status":       "failed",
				"message":      "任务执行失败 (退出码: -1)",
				"target_hosts": "real-host-77",
				"duration_ms":  int64(1500),
				"run": map[string]interface{}{
					"run_id":    "run-77",
					"status":    "failed",
					"exit_code": -1,
					"stats": map[string]interface{}{
						"failed": 1,
					},
				},
			},
		},
	}

	variables := executor.buildNotificationVariables(instance, true, true)
	execution := variables["execution"].(map[string]interface{})
	errorVars := variables["error"].(map[string]interface{})
	stats := variables["stats"].(map[string]interface{})

	if variables["flow_status"] != "failed" {
		t.Fatalf("flow_status = %v, want failed", variables["flow_status"])
	}
	if execution["run_id"] != "run-77" {
		t.Fatalf("execution.run_id = %v, want run-77", execution["run_id"])
	}
	if execution["message"] != "任务执行失败 (退出码: -1)" {
		t.Fatalf("execution.message = %v", execution["message"])
	}
	if errorVars["message"] != "任务执行失败 (退出码: -1)" {
		t.Fatalf("error.message = %v", errorVars["message"])
	}
	if errorVars["host"] != "real-host-77" {
		t.Fatalf("error.host = %v, want real-host-77", errorVars["host"])
	}
	if stats["failed"] != 1 {
		t.Fatalf("stats.failed = %v, want 1", stats["failed"])
	}
}

func TestNotificationErrorMessageFallsBackToInstanceError(t *testing.T) {
	instance := &model.FlowInstance{
		Status:       model.FlowInstanceStatusFailed,
		ErrorMessage: "实例执行失败",
		Context: model.JSON{
			"execution_result": map[string]interface{}{
				"status": "failed",
			},
		},
	}

	if got := notificationErrorMessage(instance); got != "实例执行失败" {
		t.Fatalf("notificationErrorMessage() = %q, want 实例执行失败", got)
	}
}

func TestBuildNotificationStatsMapSupportsModelJSON(t *testing.T) {
	stats := buildNotificationStatsMap(model.JSON{
		"ok":          28,
		"changed":     6,
		"failed":      0,
		"unreachable": 0,
		"skipped":     6,
	})

	if stats["ok"] != 28 {
		t.Fatalf("stats.ok = %v, want 28", stats["ok"])
	}
	if stats["changed"] != 6 {
		t.Fatalf("stats.changed = %v, want 6", stats["changed"])
	}
	if stats["total"] != 40 {
		t.Fatalf("stats.total = %v, want 40", stats["total"])
	}
	if stats["success_rate"] != "85%" {
		t.Fatalf("stats.success_rate = %v, want 85%%", stats["success_rate"])
	}
}
