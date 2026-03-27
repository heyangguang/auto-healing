package schedule

import (
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/robfig/cron/v3"
)

type UpdateInput struct {
	Name                *string
	Description         *string
	ScheduleType        *string
	ScheduleExpr        *string
	ScheduledAt         *time.Time
	TargetHostsOverride *string
	ExtraVarsOverride   *model.JSON
	SecretsSourceIDs    *model.StringArray
	SkipNotification    *bool
	MaxFailures         *int
}

func (s *Service) applyUpdate(schedule *model.ExecutionSchedule, input *UpdateInput) error {
	if input == nil {
		return nil
	}

	applyOptionalText(&schedule.Name, input.Name)
	applyOptionalText(&schedule.Description, input.Description)
	applyScheduleModeUpdate(schedule, input.ScheduleType)
	applyOptionalScheduleExpr(schedule, input.ScheduleExpr)
	applyOptionalScheduleTime(schedule, input.ScheduledAt)
	applyOptionalText(&schedule.TargetHostsOverride, input.TargetHostsOverride)
	applyOptionalJSON(&schedule.ExtraVarsOverride, input.ExtraVarsOverride)
	applyOptionalSecrets(&schedule.SecretsSourceIDs, input.SecretsSourceIDs)
	applyOptionalBool(&schedule.SkipNotification, input.SkipNotification)

	if err := applyOptionalMaxFailures(&schedule.MaxFailures, input.MaxFailures); err != nil {
		return err
	}
	return s.prepareScheduleForSave(schedule)
}

func applyOptionalText(target *string, value *string) {
	if value != nil {
		*target = *value
	}
}

func applyScheduleModeUpdate(schedule *model.ExecutionSchedule, nextType *string) {
	if nextType == nil || *nextType == "" || *nextType == schedule.ScheduleType {
		return
	}

	schedule.ScheduleType = *nextType
	if *nextType == model.ScheduleTypeCron {
		schedule.ScheduledAt = nil
		return
	}
	if *nextType == model.ScheduleTypeOnce {
		schedule.ScheduleExpr = nil
	}
}

func applyOptionalScheduleExpr(schedule *model.ExecutionSchedule, expr *string) {
	if expr != nil {
		schedule.ScheduleExpr = expr
	}
}

func applyOptionalScheduleTime(schedule *model.ExecutionSchedule, scheduledAt *time.Time) {
	if scheduledAt != nil {
		schedule.ScheduledAt = scheduledAt
	}
}

func applyOptionalJSON(target *model.JSON, value *model.JSON) {
	if value != nil {
		*target = *value
	}
}

func applyOptionalSecrets(target *model.StringArray, value *model.StringArray) {
	if value != nil {
		*target = *value
	}
}

func applyOptionalBool(target *bool, value *bool) {
	if value != nil {
		*target = *value
	}
}

func applyOptionalMaxFailures(target *int, value *int) error {
	if value == nil {
		return nil
	}
	if *value < 0 {
		return fmt.Errorf("最大连续失败次数不能为负数")
	}
	*target = *value
	return nil
}

func (s *Service) prepareScheduleForSave(schedule *model.ExecutionSchedule) error {
	normalizeScheduleModeFields(schedule)
	if err := validateScheduleDefinition(schedule); err != nil {
		return err
	}
	if !schedule.Enabled {
		schedule.NextRunAt = nil
		return nil
	}
	return s.validateAndSetNextRun(schedule)
}

func validateScheduleDefinition(schedule *model.ExecutionSchedule) error {
	switch schedule.ScheduleType {
	case model.ScheduleTypeCron:
		if schedule.ScheduleExpr == nil || *schedule.ScheduleExpr == "" {
			return fmt.Errorf("循环调度必须提供 Cron 表达式")
		}
		if _, err := cron.ParseStandard(*schedule.ScheduleExpr); err != nil {
			return fmt.Errorf("无效的 Cron 表达式: %w", err)
		}
		return nil
	case model.ScheduleTypeOnce:
		if schedule.ScheduledAt == nil {
			return fmt.Errorf("单次调度必须提供执行时间")
		}
		return nil
	default:
		return fmt.Errorf("无效的调度类型: %s（支持: cron, once）", schedule.ScheduleType)
	}
}

func normalizeScheduleModeFields(schedule *model.ExecutionSchedule) {
	switch schedule.ScheduleType {
	case model.ScheduleTypeCron:
		schedule.ScheduledAt = nil
	case model.ScheduleTypeOnce:
		schedule.ScheduleExpr = nil
	}
}
