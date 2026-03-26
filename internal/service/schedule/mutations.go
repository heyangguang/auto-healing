package schedule

import "github.com/company/auto-healing/internal/model"

func buildScheduleDefinitionUpdates(schedule *model.ExecutionSchedule) map[string]any {
	return map[string]any{
		"name":                  schedule.Name,
		"description":           schedule.Description,
		"schedule_type":         schedule.ScheduleType,
		"schedule_expr":         schedule.ScheduleExpr,
		"scheduled_at":          schedule.ScheduledAt,
		"next_run_at":           schedule.NextRunAt,
		"max_failures":          schedule.MaxFailures,
		"target_hosts_override": schedule.TargetHostsOverride,
		"extra_vars_override":   schedule.ExtraVarsOverride,
		"secrets_source_ids":    schedule.SecretsSourceIDs,
		"skip_notification":     schedule.SkipNotification,
	}
}

func buildScheduleEnableUpdates(schedule *model.ExecutionSchedule) map[string]any {
	updates := map[string]any{
		"enabled":              schedule.Enabled,
		"status":               schedule.Status,
		"next_run_at":          schedule.NextRunAt,
		"consecutive_failures": schedule.ConsecutiveFailures,
		"pause_reason":         schedule.PauseReason,
	}
	if schedule.IsOnce() && schedule.LastRunAt == nil {
		updates["last_run_at"] = nil
	}
	return updates
}

func buildScheduleDisableUpdates(schedule *model.ExecutionSchedule) map[string]any {
	return map[string]any{
		"enabled":     schedule.Enabled,
		"status":      schedule.Status,
		"next_run_at": schedule.NextRunAt,
	}
}

func buildScheduleCompletionUpdates(schedule *model.ExecutionSchedule) map[string]any {
	return map[string]any{
		"enabled": schedule.Enabled,
		"status":  schedule.Status,
	}
}
