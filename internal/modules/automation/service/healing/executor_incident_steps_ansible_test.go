package healing

import (
	"strings"
	"testing"
)

func TestParseAnsibleTaskResults(t *testing.T) {
	stdout := strings.Join([]string{
		"PLAY [Fault recovery suite workflow] *******************************************",
		"TASK [Initialize fault lab context] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [fault_lab_context : Ensure suite report directory exists] ****************",
		"changed: [192.168.31.100]",
		"TASK [fault_lab_service : Wait for lab service port] ***************************",
		"ok: [192.168.31.100]",
		"PLAY RECAP *********************************************************************",
	}, "\n")

	results := parseAnsibleTaskResults(stdout)
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if results[0].Name != "Initialize fault lab context" || results[0].Status != "ok" {
		t.Fatalf("results[0] = %#v", results[0])
	}
	if results[1].Status != "changed" {
		t.Fatalf("results[1] = %#v", results[1])
	}
}

func TestParseAnsibleTaskResultsSkipsPseudoTasksAndCapturesDetail(t *testing.T) {
	stdout := strings.Join([]string{
		"TASK [Collect scenario diagnostics] ********************************************",
		"TASK [fault_lab_diagnose : Collect service diagnostics] ************************",
		"included: /tmp/auto-healing/exec/run/playbooks/roles/fault_lab_diagnose/tasks/service_down.yml for 192.168.31.100",
		"TASK [fault_lab_verify : Assert service recovery succeeded] ********************",
		`ok: [192.168.31.100] => {`,
		`    "changed": false,`,
		`    "msg": "All assertions passed"`,
		`}`,
	}, "\n")

	results := parseAnsibleTaskResults(stdout)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Status != "included" || results[0].Detail != "service_down.yml" {
		t.Fatalf("results[0] = %#v", results[0])
	}
	if results[1].Detail != "All assertions passed" {
		t.Fatalf("results[1] = %#v", results[1])
	}
}

func TestAutoCloseExecutionTaskDetail(t *testing.T) {
	stdout := strings.Join([]string{
		"TASK [Initialize fault lab context] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [fault_lab_service : Start and enable lab service] ************************",
		"changed: [192.168.31.100]",
	}, "\n")

	detail := autoCloseExecutionTaskDetail(stdout)
	if !strings.Contains(detail, "Initialize fault lab context：成功") {
		t.Fatalf("detail = %q", detail)
	}
	if !strings.Contains(detail, "Start and enable lab service：已变更") {
		t.Fatalf("detail = %q", detail)
	}
}

func TestAutoCloseExecutionTaskDetailDoesNotTruncateTasks(t *testing.T) {
	stdout := strings.Join([]string{
		"TASK [task 1] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [task 2] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [task 3] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [task 4] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [task 5] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [task 6] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [task 7] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [task 8] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [task 9] ********************************************",
		"ok: [192.168.31.100]",
		"TASK [task 10] *******************************************",
		"ok: [192.168.31.100]",
		"TASK [task 11] *******************************************",
		"ok: [192.168.31.100]",
	}, "\n")

	detail := autoCloseExecutionTaskDetail(stdout)
	if strings.Contains(detail, "其余步骤省略") {
		t.Fatalf("detail = %q, should not truncate tasks", detail)
	}
	if !strings.Contains(detail, "task 11：成功") {
		t.Fatalf("detail = %q, want task 11", detail)
	}
}
