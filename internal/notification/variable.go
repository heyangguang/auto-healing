package notification

import (
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
)

// VariableBuilder 变量构建器
// 从执行记录、任务等数据构建模板变量
type VariableBuilder struct {
	systemName    string
	systemURL     string
	systemVersion string
}

// NewVariableBuilder 创建变量构建器
func NewVariableBuilder(systemName, systemURL, systemVersion string) *VariableBuilder {
	if systemName == "" {
		systemName = "Auto-Healing"
	}
	if systemVersion == "" {
		systemVersion = "1.0.0"
	}
	return &VariableBuilder{
		systemName:    systemName,
		systemURL:     systemURL,
		systemVersion: systemVersion,
	}
}

// BuildFromExecution 从执行记录构建变量
func (b *VariableBuilder) BuildFromExecution(run *model.ExecutionRun, task *model.ExecutionTask) map[string]interface{} {
	now := time.Now()
	vars := map[string]interface{}{
		"timestamp": now.Format("2006-01-02 15:04:05"),
		"date":      now.Format("2006-01-02"),
		"time":      now.Format("15:04:05"),
	}

	// execution.*
	execution := map[string]interface{}{
		"run_id":           run.ID.String(),
		"status":           run.Status,
		"status_emoji":     b.getStatusEmoji(run.Status),
		"triggered_by":     run.TriggeredBy,
		"trigger_type":     b.getTriggerType(run.TriggeredBy),
		"started_at":       b.formatTime(run.StartedAt),
		"completed_at":     b.formatTime(run.CompletedAt),
		"duration":         b.calculateDuration(run.StartedAt, run.CompletedAt),
		"duration_seconds": b.calculateDurationSeconds(run.StartedAt, run.CompletedAt),
		"stdout":           b.truncateText(run.Stdout, 100000), // Ansible 标准输出
		"stderr":           b.truncateText(run.Stderr, 100000), // Ansible 错误输出
	}
	if run.ExitCode != nil {
		execution["exit_code"] = *run.ExitCode
	} else {
		execution["exit_code"] = ""
	}
	vars["execution"] = execution

	// task.*
	hostList := strings.Split(task.TargetHosts, ",")
	taskVars := map[string]interface{}{
		"id":            task.ID.String(),
		"name":          task.Name,
		"target_hosts":  task.TargetHosts,
		"host_count":    len(hostList),
		"executor_type": task.ExecutorType,
	}
	vars["task"] = taskVars

	// repository.* / playbook.*
	if task.Playbook != nil {
		playbook := task.Playbook
		playbookVars := map[string]interface{}{
			"id":        playbook.ID.String(),
			"name":      playbook.Name,
			"file_path": playbook.FilePath,
			"status":    playbook.Status,
		}
		vars["playbook"] = playbookVars

		// 仓库信息（如果有）
		if playbook.Repository != nil {
			repo := playbook.Repository
			repoVars := map[string]interface{}{
				"id":       repo.ID.String(),
				"name":     repo.Name,
				"url":      repo.URL,
				"branch":   repo.DefaultBranch,
				"playbook": playbook.FilePath,
			}
			vars["repository"] = repoVars
		} else {
			vars["repository"] = map[string]interface{}{
				"id":       "",
				"name":     "",
				"url":      "",
				"branch":   "",
				"playbook": playbook.FilePath,
			}
		}
	} else {
		vars["playbook"] = map[string]interface{}{
			"id":        "",
			"name":      "",
			"file_path": "",
			"status":    "",
		}
		vars["repository"] = map[string]interface{}{
			"id":       "",
			"name":     "",
			"url":      "",
			"branch":   "",
			"playbook": "",
		}
	}

	// stats.*
	stats := b.parseStats(run.Stats)
	vars["stats"] = stats

	// error.*
	errorVars := b.parseError(run)
	vars["error"] = errorVars

	// system.*
	vars["system"] = map[string]interface{}{
		"name":    b.systemName,
		"url":     b.systemURL,
		"version": b.systemVersion,
		"env":     "production",
	}

	return vars
}

// getStatusEmoji 获取状态表情
func (b *VariableBuilder) getStatusEmoji(status string) string {
	switch status {
	case "success":
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

// getTriggerType 获取触发类型
func (b *VariableBuilder) getTriggerType(triggeredBy string) string {
	if triggeredBy == "" {
		return "manual"
	}
	if strings.HasPrefix(triggeredBy, "scheduler") || strings.Contains(triggeredBy, "定时") {
		return "scheduled"
	}
	if strings.HasPrefix(triggeredBy, "workflow") {
		return "workflow"
	}
	return "manual"
}

// formatTime 格式化时间
func (b *VariableBuilder) formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// calculateDuration 计算执行时长（格式化）
func (b *VariableBuilder) calculateDuration(start, end *time.Time) string {
	if start == nil || end == nil {
		return ""
	}
	duration := end.Sub(*start)

	if duration < time.Minute {
		return fmt.Sprintf("%.0fs", duration.Seconds())
	}
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// calculateDurationSeconds 计算执行时长（秒）
func (b *VariableBuilder) calculateDurationSeconds(start, end *time.Time) int {
	if start == nil || end == nil {
		return 0
	}
	return int(end.Sub(*start).Seconds())
}

// truncateText 截断文本（用于日志输出）
func (b *VariableBuilder) truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "\n... (输出已截断)"
}

// parseStats 解析 Ansible 统计信息
func (b *VariableBuilder) parseStats(statsJSON model.JSON) map[string]interface{} {
	stats := map[string]interface{}{
		"ok":           0,
		"changed":      0,
		"failed":       0,
		"unreachable":  0,
		"skipped":      0,
		"rescued":      0,
		"ignored":      0,
		"total":        0,
		"success_rate": "100%",
	}

	if statsJSON == nil {
		return stats
	}

	ok := 0
	changed := 0
	failed := 0
	unreachable := 0
	skipped := 0
	rescued := 0
	ignored := 0

	// 先尝试直接读取平坦化的 stats（来自我们的保存格式）
	if v, ok := statsJSON["ok"].(float64); ok {
		stats["ok"] = int(v)
	}
	if v, ok := statsJSON["changed"].(float64); ok {
		changed = int(v)
		stats["changed"] = changed
	}
	if v, ok := statsJSON["failed"].(float64); ok {
		failed = int(v)
		stats["failed"] = failed
	}
	if v, ok := statsJSON["unreachable"].(float64); ok {
		unreachable = int(v)
		stats["unreachable"] = unreachable
	}
	if v, ok := statsJSON["skipped"].(float64); ok {
		skipped = int(v)
		stats["skipped"] = skipped
	}
	if v, ok := statsJSON["rescued"].(float64); ok {
		rescued = int(v)
		stats["rescued"] = rescued
	}
	if v, ok := statsJSON["ignored"].(float64); ok {
		ignored = int(v)
		stats["ignored"] = ignored
	}

	// 计算总数和成功率
	if okVal, exists := stats["ok"].(int); exists {
		ok = okVal
	}
	total := ok + changed + failed + unreachable + skipped
	stats["total"] = total

	if total > 0 {
		successCount := ok + changed
		rate := float64(successCount) / float64(total) * 100
		stats["success_rate"] = fmt.Sprintf("%.0f%%", rate)
	}

	return stats
}

// parseError 解析错误信息
func (b *VariableBuilder) parseError(run *model.ExecutionRun) map[string]interface{} {
	errorVars := map[string]interface{}{
		"message": "",
		"host":    "",
	}

	if run.Status != "failed" && run.Status != "timeout" {
		return errorVars
	}

	// 从 stderr 提取错误信息（取前 500 字符）
	if run.Stderr != "" {
		msg := run.Stderr
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		errorVars["message"] = msg
	}

	// 尝试从 stats 中找出失败的主机
	if run.Stats != nil {
		for host, hostStats := range run.Stats {
			if hs, ok := hostStats.(map[string]interface{}); ok {
				failures, _ := hs["failures"].(float64)
				unreachable, _ := hs["unreachable"].(float64)
				if failures > 0 || unreachable > 0 {
					errorVars["host"] = host
					break
				}
			}
		}
	}

	return errorVars
}
