package healing

import (
	"fmt"
	"time"

	cfg "github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

func (e *FlowExecutor) buildNotificationVariables(instance *model.FlowInstance, includeExecutionResult, includeIncidentInfo bool) map[string]interface{} {
	variables := make(map[string]interface{})
	now := time.Now()
	addNotificationTimeVariables(variables, now)
	addNotificationFlowVariables(variables, instance)
	addNotificationSystemVariables(variables)
	e.addNotificationIncidentVariables(variables, instance, includeIncidentInfo)
	e.addNotificationExecutionVariables(variables, instance, includeExecutionResult)
	addNotificationTaskVariables(variables, instance)
	addNotificationRepositoryVariables(variables, instance)
	addNotificationErrorVariables(variables, instance)
	addNotificationValidationVariables(variables, instance)
	addNotificationHostVariables(variables, instance)
	logger.Exec("NODE").Debug("通知变量: %v", variables)
	return variables
}

func addNotificationTimeVariables(variables map[string]interface{}, now time.Time) {
	variables["timestamp"] = now.Format("2006-01-02 15:04:05")
	variables["date"] = now.Format("2006-01-02")
	variables["time"] = now.Format("15:04:05")
}

func addNotificationFlowVariables(variables map[string]interface{}, instance *model.FlowInstance) {
	variables["flow_instance_id"] = instance.ID
	variables["flow_status"] = instance.Status
}

func addNotificationSystemVariables(variables map[string]interface{}) {
	appCfg := cfg.GetAppConfig()
	variables["system"] = map[string]interface{}{
		"name":    appCfg.Name,
		"url":     appCfg.URL,
		"version": appCfg.Version,
		"env":     appCfg.Env,
	}
	variables["system_name"] = appCfg.Name
	variables["system_version"] = appCfg.Version
	variables["system_env"] = appCfg.Env
}

func (e *FlowExecutor) addNotificationIncidentVariables(variables map[string]interface{}, instance *model.FlowInstance, includeIncidentInfo bool) {
	if !includeIncidentInfo || instance.Context == nil {
		return
	}
	incident, ok := instance.Context["incident"].(map[string]interface{})
	if !ok {
		return
	}
	variables["incident_id"] = incident["id"]
	variables["incident_title"] = incident["title"]
	variables["incident_severity"] = incident["severity"]
	variables["incident_source"] = incident["source_plugin_name"]
	variables["incident_external_id"] = incident["external_id"]
	variables["incident_status"] = incident["status"]
	for key, value := range incident {
		variables["incident_"+key] = value
	}
}

func (e *FlowExecutor) addNotificationExecutionVariables(variables map[string]interface{}, instance *model.FlowInstance, includeExecutionResult bool) {
	executionMap := map[string]interface{}{
		"run_id":           instance.ID.String(),
		"status":           "",
		"status_emoji":     "❓",
		"exit_code":        "",
		"triggered_by":     "workflow",
		"trigger_type":     "workflow",
		"started_at":       "",
		"completed_at":     "",
		"duration":         "",
		"duration_seconds": 0,
		"stdout":           "",
		"stderr":           "",
	}
	if includeExecutionResult && instance.Context != nil {
		if result, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			fillNotificationExecutionMap(variables, executionMap, result)
		}
	}
	variables["execution"] = executionMap
}

func fillNotificationExecutionMap(variables map[string]interface{}, executionMap map[string]interface{}, result map[string]interface{}) {
	executionMap["status"] = result["status"]
	executionMap["exit_code"] = result["exit_code"]
	executionMap["stdout"] = result["stdout"]
	executionMap["stderr"] = result["stderr"]
	fillNotificationDuration(executionMap, result["duration_ms"])
	executionMap["started_at"] = result["started_at"]
	executionMap["completed_at"] = result["finished_at"]
	executionMap["status_emoji"] = executionStatusEmoji(fmt.Sprintf("%v", result["status"]))

	variables["execution_status"] = result["status"]
	variables["execution_message"] = result["message"]
	variables["execution_exit_code"] = result["exit_code"]
	variables["execution_stdout"] = result["stdout"]
	variables["execution_stderr"] = result["stderr"]
	variables["execution_duration_ms"] = result["duration_ms"]
	variables["execution_playbook_path"] = result["playbook_path"]
	variables["execution_status_emoji"] = executionMap["status_emoji"]

	if statsRaw, ok := result["stats"]; ok && statsRaw != nil {
		variables["stats"] = buildNotificationStatsMap(statsRaw)
	}
	for key, value := range result {
		if _, exists := variables["execution_"+key]; !exists {
			variables["execution_"+key] = value
		}
	}
}

func fillNotificationDuration(executionMap map[string]interface{}, raw interface{}) {
	durationMs := int64(0)
	switch typed := raw.(type) {
	case int64:
		durationMs = typed
	case float64:
		durationMs = int64(typed)
	case int:
		durationMs = int64(typed)
	default:
		return
	}
	executionMap["duration_seconds"] = int(durationMs / 1000)
	if durationMs < 60000 {
		executionMap["duration"] = fmt.Sprintf("%ds", durationMs/1000)
		return
	}
	minutes := durationMs / 60000
	seconds := (durationMs % 60000) / 1000
	executionMap["duration"] = fmt.Sprintf("%dm %ds", minutes, seconds)
}

func executionStatusEmoji(status string) string {
	switch status {
	case "completed", "success":
		return "✅"
	case "failed":
		return "❌"
	case "timeout":
		return "⏱️"
	case "cancelled":
		return "🚫"
	case "running":
		return "🔄"
	default:
		return "❓"
	}
}

func buildNotificationStatsMap(statsRaw interface{}) map[string]interface{} {
	statsMap := map[string]interface{}{
		"ok": 0, "changed": 0, "failed": 0, "unreachable": 0,
		"skipped": 0, "rescued": 0, "ignored": 0, "total": 0, "success_rate": "N/A",
	}
	switch stats := statsRaw.(type) {
	case map[string]int:
		statsMap["ok"] = stats["ok"]
		statsMap["changed"] = stats["changed"]
		statsMap["failed"] = stats["failed"]
		statsMap["unreachable"] = stats["unreachable"]
		statsMap["skipped"] = stats["skipped"]
		statsMap["rescued"] = stats["rescued"]
		statsMap["ignored"] = stats["ignored"]
		total := stats["ok"] + stats["changed"] + stats["failed"] + stats["unreachable"] + stats["skipped"]
		statsMap["total"] = total
		if total > 0 {
			statsMap["success_rate"] = fmt.Sprintf("%.0f%%", float64(stats["ok"]+stats["changed"])/float64(total)*100)
		}
	case map[string]interface{}:
		statsMap["ok"] = stats["ok"]
		statsMap["changed"] = stats["changed"]
		statsMap["failed"] = stats["failed"]
		statsMap["unreachable"] = stats["unreachable"]
		statsMap["skipped"] = stats["skipped"]
		statsMap["rescued"] = stats["rescued"]
		statsMap["ignored"] = stats["ignored"]
		okCount := toFloat(stats["ok"])
		changedCount := toFloat(stats["changed"])
		failedCount := toFloat(stats["failed"])
		unreachableCount := toFloat(stats["unreachable"])
		skippedCount := toFloat(stats["skipped"])
		total := okCount + changedCount + failedCount + unreachableCount + skippedCount
		statsMap["total"] = int(total)
		if total > 0 {
			statsMap["success_rate"] = fmt.Sprintf("%.0f%%", (okCount+changedCount)/total*100)
		}
	}
	return statsMap
}

func addNotificationTaskVariables(variables map[string]interface{}, instance *model.FlowInstance) {
	taskMap := map[string]interface{}{
		"id":            instance.ID.String(),
		"name":          fmt.Sprintf("流程实例 #%s", instance.ID.String()[:8]),
		"target_hosts":  "",
		"host_count":    0,
		"executor_type": "local",
		"is_recurring":  false,
	}
	variables["task"] = taskMap
}

func addNotificationRepositoryVariables(variables map[string]interface{}, instance *model.FlowInstance) {
	repoMap := map[string]interface{}{
		"id":            "",
		"name":          "",
		"url":           "",
		"branch":        "",
		"playbook":      "",
		"main_playbook": "",
	}
	if instance.Context != nil {
		if execResult, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			if playbookPath, ok := execResult["playbook_path"].(string); ok {
				repoMap["playbook"] = playbookPath
				repoMap["main_playbook"] = playbookPath
			}
		}
	}
	variables["repository"] = repoMap
}

func addNotificationErrorVariables(variables map[string]interface{}, instance *model.FlowInstance) {
	errorMap := map[string]interface{}{"message": "", "host": ""}
	if instance.Context != nil {
		if execResult, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			status := fmt.Sprintf("%v", execResult["status"])
			if (status == "failed" || status == "timeout") && execResult["stderr"] != nil {
				if stderr, ok := execResult["stderr"].(string); ok && stderr != "" {
					if len(stderr) > 500 {
						stderr = stderr[:500] + "..."
					}
					errorMap["message"] = stderr
				}
			}
		}
	}
	variables["error"] = errorMap
}

func addNotificationValidationVariables(variables map[string]interface{}, instance *model.FlowInstance) {
	validationMap := map[string]interface{}{"total": 0, "matched": 0, "unmatched": 0}
	if instance.Context != nil {
		if summary, ok := instance.Context["validation_summary"].(map[string]interface{}); ok {
			validationMap["total"] = summary["total"]
			validationMap["matched"] = summary["valid"]
			validationMap["unmatched"] = summary["invalid"]
		}
	}
	variables["validation"] = validationMap
}

func addNotificationHostVariables(variables map[string]interface{}, instance *model.FlowInstance) {
	hostCount := 0
	if instance.Context != nil {
		if hosts, ok := instance.Context["validated_hosts"]; ok {
			variables["target_hosts"] = hosts
			switch typed := hosts.(type) {
			case []interface{}:
				hostCount = len(typed)
			case []map[string]interface{}:
				hostCount = len(typed)
			case []string:
				hostCount = len(typed)
			default:
				hostCount = 1
			}
		}
	}
	variables["host_count"] = hostCount
	if task, ok := variables["task"].(map[string]interface{}); ok {
		task["host_count"] = hostCount
	}
}
