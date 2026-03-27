package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"github.com/google/uuid"
)

func TestVariableBuilderBuildFromExecution(t *testing.T) {
	builder := NewVariableBuilder("", "https://auto-healing.example", "")
	startedAt := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(2*time.Minute + 5*time.Second)
	exitCode := 2

	run := &projection.ExecutionRun{
		ID:          uuid.New(),
		Status:      "failed",
		TriggeredBy: "scheduler-nightly",
		ExitCode:    &exitCode,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		Stdout:      strings.Repeat("x", executionOutputMaxLen+10),
		Stderr:      "playbook failed",
		Stats: model.JSON{
			"ok":         float64(1),
			"changed":    float64(1),
			"failed":     float64(1),
			"app-host-1": map[string]interface{}{"failures": float64(1), "unreachable": float64(0)},
		},
	}
	task := &projection.ExecutionTask{
		ID:           uuid.New(),
		Name:         "Deploy App",
		TargetHosts:  "app-1, app-2, , ",
		ExecutorType: "docker",
		Playbook: &projection.Playbook{
			ID:       uuid.New(),
			Name:     "deploy.yml",
			FilePath: "playbooks/deploy.yml",
			Status:   "ready",
			Repository: &projection.GitRepository{
				ID:            uuid.New(),
				Name:          "infra",
				URL:           "https://git.example/infra.git",
				DefaultBranch: "main",
			},
		},
	}

	vars := builder.BuildFromExecution(run, task)
	execution := vars["execution"].(map[string]interface{})
	taskVars := vars["task"].(map[string]interface{})
	repoVars := vars["repository"].(map[string]interface{})
	errorVars := vars["error"].(map[string]interface{})
	systemVars := vars["system"].(map[string]interface{})

	if execution["status_emoji"] != "❌" {
		t.Fatalf("status_emoji = %v, 期望 ❌", execution["status_emoji"])
	}
	if execution["trigger_type"] != "scheduled" {
		t.Fatalf("trigger_type = %v, 期望 scheduled", execution["trigger_type"])
	}
	if execution["duration"] != "2m 5s" {
		t.Fatalf("duration = %v, 期望 2m 5s", execution["duration"])
	}
	if taskVars["host_count"] != 2 {
		t.Fatalf("host_count = %v, 期望 2", taskVars["host_count"])
	}
	if repoVars["playbook"] != "playbooks/deploy.yml" {
		t.Fatalf("repository.playbook = %v, 期望 playbooks/deploy.yml", repoVars["playbook"])
	}
	if errorVars["host"] != "app-host-1" {
		t.Fatalf("error.host = %v, 期望 app-host-1", errorVars["host"])
	}
	if systemVars["name"] != defaultSystemName {
		t.Fatalf("system.name = %v, 期望 %s", systemVars["name"], defaultSystemName)
	}
}

func TestVariableBuilderParseStatsDefaults(t *testing.T) {
	builder := NewVariableBuilder("Auto-Healing", "", "1.2.3")
	stats := builder.parseStats(model.JSON{
		"ok":          float64(2),
		"changed":     float64(1),
		"failed":      float64(1),
		"unreachable": float64(0),
		"skipped":     float64(0),
	})

	if stats["total"] != 4 {
		t.Fatalf("total = %v, 期望 4", stats["total"])
	}
	if stats["success_rate"] != "75%" {
		t.Fatalf("success_rate = %v, 期望 75%%", stats["success_rate"])
	}
}
