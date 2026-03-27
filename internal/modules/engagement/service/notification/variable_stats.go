package notification

import (
	"fmt"

	"github.com/company/auto-healing/internal/modules/engagement/model"
)

const (
	errorPreviewMaxLen = 500
	statsPercentScale  = 100
	secondsPerMinute   = 60
	minutesPerHour     = 60
)

func (b *VariableBuilder) parseStats(statsJSON model.JSON) map[string]interface{} {
	stats := defaultStatsMap()
	if statsJSON == nil {
		return stats
	}

	applyFlatStats(stats, statsJSON)
	fillDerivedStats(stats)
	return stats
}

func defaultStatsMap() map[string]interface{} {
	return map[string]interface{}{
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
}

func applyFlatStats(stats map[string]interface{}, statsJSON model.JSON) {
	for _, key := range []string{"ok", "changed", "failed", "unreachable", "skipped", "rescued", "ignored"} {
		if value, ok := statsJSON[key].(float64); ok {
			stats[key] = int(value)
		}
	}
}

func fillDerivedStats(stats map[string]interface{}) {
	okCount := stats["ok"].(int)
	changedCount := stats["changed"].(int)
	failedCount := stats["failed"].(int)
	unreachableCount := stats["unreachable"].(int)
	skippedCount := stats["skipped"].(int)
	total := okCount + changedCount + failedCount + unreachableCount + skippedCount

	stats["total"] = total
	if total == 0 {
		return
	}

	successCount := okCount + changedCount
	rate := float64(successCount) / float64(total) * statsPercentScale
	stats["success_rate"] = fmt.Sprintf("%.0f%%", rate)
}

func (b *VariableBuilder) parseError(run *model.ExecutionRun) map[string]interface{} {
	errorVars := map[string]interface{}{
		"message": "",
		"host":    "",
	}
	if run.Status != "failed" && run.Status != "timeout" {
		return errorVars
	}

	errorVars["message"] = truncatedErrorMessage(run.Stderr)
	errorVars["host"] = firstFailedHost(run.Stats)
	return errorVars
}

func truncatedErrorMessage(stderr string) string {
	if stderr == "" {
		return ""
	}
	if len(stderr) <= errorPreviewMaxLen {
		return stderr
	}
	return stderr[:errorPreviewMaxLen] + "..."
}

func firstFailedHost(stats model.JSON) string {
	if stats == nil {
		return ""
	}

	for host, hostStats := range stats {
		values, ok := hostStats.(map[string]interface{})
		if !ok {
			continue
		}
		if values["failures"].(float64) > 0 || values["unreachable"].(float64) > 0 {
			return host
		}
	}
	return ""
}
