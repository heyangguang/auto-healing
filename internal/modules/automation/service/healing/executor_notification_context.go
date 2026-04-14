package healing

import (
	"strings"

	"github.com/company/auto-healing/internal/modules/automation/model"
)

const notificationErrorMessageMaxLen = 500

func derivedNotificationFlowStatus(instance *model.FlowInstance) string {
	if instance == nil {
		return ""
	}
	status := strings.TrimSpace(instance.Status)
	switch notificationExecutionStatus(notificationExecutionResult(instance)) {
	case "failed", "partial", "timeout", "cancelled":
		return notificationExecutionStatus(notificationExecutionResult(instance))
	case "completed", "success":
		if status == "" || status == model.FlowInstanceStatusRunning {
			return model.FlowInstanceStatusCompleted
		}
	}
	return status
}

func notificationExecutionResult(instance *model.FlowInstance) map[string]interface{} {
	if instance == nil || instance.Context == nil {
		return nil
	}
	result, _ := instance.Context["execution_result"].(map[string]interface{})
	return result
}

func notificationExecutionRun(result map[string]interface{}) map[string]interface{} {
	if result == nil {
		return nil
	}
	run, _ := result["run"].(map[string]interface{})
	return run
}

func notificationExecutionStatus(result map[string]interface{}) string {
	if runStatus := stringValue(notificationExecutionRun(result)["status"]); runStatus != "" {
		return runStatus
	}
	return stringValue(result["status"])
}

func notificationExecutionRunID(instance *model.FlowInstance, result map[string]interface{}) string {
	if runID := stringValue(notificationExecutionRun(result)["run_id"]); runID != "" {
		return runID
	}
	if runID := stringValue(result["run_id"]); runID != "" {
		return runID
	}
	if instance == nil {
		return ""
	}
	return instance.ID.String()
}

func notificationExecutionMessage(result map[string]interface{}) string {
	for _, candidate := range []string{
		stringValue(result["message"]),
		stringValue(result["error_message"]),
		stringValue(notificationExecutionRun(result)["stderr"]),
		stringValue(result["stderr"]),
	} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func notificationExecutionExitCode(result map[string]interface{}) interface{} {
	if exitCode := notificationExecutionRun(result)["exit_code"]; exitCode != nil {
		return exitCode
	}
	return result["exit_code"]
}

func notificationExecutionStats(result map[string]interface{}) interface{} {
	if stats := notificationExecutionRun(result)["stats"]; stats != nil {
		return stats
	}
	return result["stats"]
}

func notificationExecutionStdout(result map[string]interface{}) string {
	if stdout := stringValue(notificationExecutionRun(result)["stdout"]); stdout != "" {
		return stdout
	}
	return stringValue(result["stdout"])
}

func notificationExecutionStderr(result map[string]interface{}) string {
	if stderr := stringValue(notificationExecutionRun(result)["stderr"]); stderr != "" {
		return stderr
	}
	return stringValue(result["stderr"])
}

func notificationErrorMessage(instance *model.FlowInstance) string {
	result := notificationExecutionResult(instance)
	status := notificationExecutionStatus(result)
	if status != "failed" && status != "partial" && status != "timeout" && status != "cancelled" {
		return ""
	}
	for _, candidate := range []string{
		notificationExecutionStderr(result),
		stringValue(result["error_message"]),
		stringValue(result["message"]),
	} {
		if candidate == "" {
			continue
		}
		if len(candidate) <= notificationErrorMessageMaxLen {
			return candidate
		}
		return candidate[:notificationErrorMessageMaxLen] + "..."
	}
	if instance == nil {
		return ""
	}
	return strings.TrimSpace(instance.ErrorMessage)
}

func notificationErrorHost(instance *model.FlowInstance) string {
	result := notificationExecutionResult(instance)
	for _, candidate := range []string{
		stringValue(result["target_hosts"]),
		stringValue(result["host"]),
		stringValue(result["error_host"]),
	} {
		if candidate != "" {
			return candidate
		}
	}
	if instance == nil || instance.Context == nil {
		return ""
	}
	incident, _ := instance.Context["incident"].(map[string]interface{})
	return stringValue(incident["affected_ci"])
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
