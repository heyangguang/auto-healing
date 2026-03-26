package notification

import (
	"strings"

	"github.com/company/auto-healing/internal/model"
)

const executionOutputMaxLen = 100000

func (b *VariableBuilder) buildExecutionVariables(run *model.ExecutionRun) map[string]interface{} {
	return map[string]interface{}{
		"run_id":           run.ID.String(),
		"status":           run.Status,
		"status_emoji":     b.getStatusEmoji(run.Status),
		"triggered_by":     run.TriggeredBy,
		"trigger_type":     b.getTriggerType(run.TriggeredBy),
		"started_at":       b.formatTime(run.StartedAt),
		"completed_at":     b.formatTime(run.CompletedAt),
		"duration":         b.calculateDuration(run.StartedAt, run.CompletedAt),
		"duration_seconds": b.calculateDurationSeconds(run.StartedAt, run.CompletedAt),
		"stdout":           b.truncateText(run.Stdout, executionOutputMaxLen),
		"stderr":           b.truncateText(run.Stderr, executionOutputMaxLen),
		"exit_code":        exitCodeValue(run.ExitCode),
	}
}

func (b *VariableBuilder) buildTaskVariables(task *model.ExecutionTask) map[string]interface{} {
	hosts := splitTargetHosts(task.TargetHosts)
	return map[string]interface{}{
		"id":            task.ID.String(),
		"name":          task.Name,
		"target_hosts":  task.TargetHosts,
		"host_count":    len(hosts),
		"executor_type": task.ExecutorType,
	}
}

func (b *VariableBuilder) buildPlaybookVariables(task *model.ExecutionTask) (map[string]interface{}, map[string]interface{}) {
	if task.Playbook == nil {
		return emptyPlaybookVariables(), emptyRepositoryVariables("")
	}

	playbook := task.Playbook
	playbookVars := map[string]interface{}{
		"id":        playbook.ID.String(),
		"name":      playbook.Name,
		"file_path": playbook.FilePath,
		"status":    playbook.Status,
	}
	if playbook.Repository == nil {
		return playbookVars, emptyRepositoryVariables(playbook.FilePath)
	}

	repository := playbook.Repository
	return playbookVars, map[string]interface{}{
		"id":       repository.ID.String(),
		"name":     repository.Name,
		"url":      repository.URL,
		"branch":   repository.DefaultBranch,
		"playbook": playbook.FilePath,
	}
}

func splitTargetHosts(targetHosts string) []string {
	if strings.TrimSpace(targetHosts) == "" {
		return nil
	}

	rawHosts := strings.Split(targetHosts, ",")
	hosts := make([]string, 0, len(rawHosts))
	for _, host := range rawHosts {
		host = strings.TrimSpace(host)
		if host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func exitCodeValue(exitCode *int) interface{} {
	if exitCode == nil {
		return ""
	}
	return *exitCode
}

func emptyPlaybookVariables() map[string]interface{} {
	return map[string]interface{}{
		"id":        "",
		"name":      "",
		"file_path": "",
		"status":    "",
	}
}

func emptyRepositoryVariables(playbookPath string) map[string]interface{} {
	return map[string]interface{}{
		"id":       "",
		"name":     "",
		"url":      "",
		"branch":   "",
		"playbook": playbookPath,
	}
}
