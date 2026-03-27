package schedule

import (
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
)

func TestApplyUpdatePreservesOmittedFields(t *testing.T) {
	name := "existing"
	description := "updated"
	schedule := &model.ExecutionSchedule{
		Name:                name,
		Description:         "old",
		ScheduleType:        model.ScheduleTypeCron,
		ScheduleExpr:        stringPtr("0 * * * *"),
		Enabled:             false,
		MaxFailures:         5,
		TargetHostsOverride: "group-a",
		ExtraVarsOverride:   model.JSON{"env": "prod"},
		SecretsSourceIDs:    model.StringArray{"a"},
	}

	if err := (&Service{}).applyUpdate(schedule, &UpdateInput{Description: &description}); err != nil {
		t.Fatalf("applyUpdate() error = %v", err)
	}
	if schedule.Name != name {
		t.Fatalf("Name = %q, want %q", schedule.Name, name)
	}
	if schedule.Description != description {
		t.Fatalf("Description = %q, want %q", schedule.Description, description)
	}
	if schedule.MaxFailures != 5 {
		t.Fatalf("MaxFailures = %d, want 5", schedule.MaxFailures)
	}
	if schedule.TargetHostsOverride != "group-a" {
		t.Fatalf("TargetHostsOverride = %q, want group-a", schedule.TargetHostsOverride)
	}
}

func TestApplyUpdateRejectsNegativeMaxFailures(t *testing.T) {
	value := -1
	err := (&Service{}).applyUpdate(&model.ExecutionSchedule{}, &UpdateInput{MaxFailures: &value})
	if err == nil {
		t.Fatal("expected negative max_failures to be rejected")
	}
}

func TestApplyUpdateSwitchesScheduleMode(t *testing.T) {
	scheduledAt := time.Now().Add(time.Hour)
	schedule := &model.ExecutionSchedule{
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: stringPtr("0 * * * *"),
		Enabled:      false,
	}

	if err := (&Service{}).applyUpdate(schedule, &UpdateInput{
		ScheduleType: stringPtr(model.ScheduleTypeOnce),
		ScheduledAt:  &scheduledAt,
	}); err != nil {
		t.Fatalf("applyUpdate() error = %v", err)
	}
	if schedule.ScheduleType != model.ScheduleTypeOnce {
		t.Fatalf("ScheduleType = %q, want %q", schedule.ScheduleType, model.ScheduleTypeOnce)
	}
	if schedule.ScheduleExpr != nil {
		t.Fatal("ScheduleExpr should be cleared when switching to once")
	}
	if schedule.ScheduledAt == nil || !schedule.ScheduledAt.Equal(scheduledAt) {
		t.Fatal("ScheduledAt was not updated")
	}
}

func TestPrepareScheduleForSaveClearsNextRunWhenDisabled(t *testing.T) {
	nextRun := time.Now().Add(time.Hour)
	schedule := &model.ExecutionSchedule{
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: stringPtr("0 * * * *"),
		Enabled:      false,
		NextRunAt:    &nextRun,
	}

	if err := (&Service{}).prepareScheduleForSave(schedule); err != nil {
		t.Fatalf("prepareScheduleForSave() error = %v", err)
	}
	if schedule.NextRunAt != nil {
		t.Fatal("NextRunAt should be cleared when schedule is disabled")
	}
}

func TestPrepareScheduleForSaveRejectsInvalidTypeWhenDisabled(t *testing.T) {
	schedule := &model.ExecutionSchedule{
		ScheduleType: "invalid",
		Enabled:      false,
	}

	if err := (&Service{}).prepareScheduleForSave(schedule); err == nil {
		t.Fatal("expected invalid schedule type to be rejected")
	}
}

func TestNextRunFromExprRejectsInvalidCron(t *testing.T) {
	if _, err := (&Service{}).nextRunFromExpr("bad cron"); err == nil {
		t.Fatal("expected invalid cron expression to return an error")
	}
}

func TestPrepareScheduleForSaveClearsMutuallyExclusiveFields(t *testing.T) {
	cronExpr := "0 * * * *"
	scheduledAt := time.Now().Add(time.Hour)

	cronSchedule := &model.ExecutionSchedule{
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: &cronExpr,
		ScheduledAt:  &scheduledAt,
		Enabled:      false,
	}
	if err := (&Service{}).prepareScheduleForSave(cronSchedule); err != nil {
		t.Fatalf("prepareScheduleForSave(cron) error = %v", err)
	}
	if cronSchedule.ScheduledAt != nil {
		t.Fatal("cron schedule should clear scheduled_at")
	}

	onceSchedule := &model.ExecutionSchedule{
		ScheduleType: model.ScheduleTypeOnce,
		ScheduleExpr: &cronExpr,
		ScheduledAt:  &scheduledAt,
		Enabled:      false,
	}
	if err := (&Service{}).prepareScheduleForSave(onceSchedule); err != nil {
		t.Fatalf("prepareScheduleForSave(once) error = %v", err)
	}
	if onceSchedule.ScheduleExpr != nil {
		t.Fatal("once schedule should clear schedule_expr")
	}
}

func TestBuildScheduleDefinitionUpdatesExcludesRuntimeFields(t *testing.T) {
	schedule := &model.ExecutionSchedule{
		Name:                "nightly",
		Description:         "desc",
		ScheduleType:        model.ScheduleTypeCron,
		ScheduleExpr:        stringPtr("0 * * * *"),
		MaxFailures:         3,
		ConsecutiveFailures: 9,
		PauseReason:         "paused",
		TargetHostsOverride: "group-a",
		ExtraVarsOverride:   model.JSON{"env": "prod"},
		SecretsSourceIDs:    model.StringArray{"secret-1"},
		SkipNotification:    true,
	}

	updates := buildScheduleDefinitionUpdates(schedule)
	for _, key := range []string{"last_run_at", "consecutive_failures", "pause_reason", "status"} {
		if _, exists := updates[key]; exists {
			t.Fatalf("definition updates should not include runtime field %q", key)
		}
	}
}

func TestBuildScheduleEnableUpdatesClearsLastRunForRerunnableOnceSchedule(t *testing.T) {
	schedule := &model.ExecutionSchedule{
		ScheduleType: model.ScheduleTypeOnce,
		Enabled:      true,
		Status:       model.ScheduleStatusPending,
	}

	updates := buildScheduleEnableUpdates(schedule)
	value, exists := updates["last_run_at"]
	if !exists {
		t.Fatal("enable updates should clear last_run_at for once schedule")
	}
	if value != nil {
		t.Fatalf("last_run_at = %#v, want nil", value)
	}
}

func stringPtr(value string) *string {
	return &value
}
