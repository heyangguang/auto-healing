package notification

import (
	"fmt"
	"strings"
	"time"
)

const (
	dateTimeLayout        = "2006-01-02 15:04:05"
	dateLayout            = "2006-01-02"
	clockLayout           = "15:04:05"
	truncatedOutputSuffix = "\n... (输出已截断)"
)

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

func (b *VariableBuilder) formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(dateTimeLayout)
}

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
		seconds := int(duration.Seconds()) % secondsPerMinute
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % minutesPerHour
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func (b *VariableBuilder) calculateDurationSeconds(start, end *time.Time) int {
	if start == nil || end == nil {
		return 0
	}
	return int(end.Sub(*start).Seconds())
}

func (b *VariableBuilder) truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + truncatedOutputSuffix
}
