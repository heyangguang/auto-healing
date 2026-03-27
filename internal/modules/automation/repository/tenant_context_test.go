package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestExecutionRepositoryCreateTaskRequiresTenantContext(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			playbook_id TEXT,
			target_hosts TEXT,
			executor_type TEXT,
			needs_review BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	repo := NewExecutionRepositoryWithDB(db)
	task := &model.ExecutionTask{
		ID:           uuid.New(),
		Name:         "task",
		ExecutorType: "local",
	}

	err := repo.CreateTask(context.Background(), task)
	if !errors.Is(err, platformrepo.ErrTenantContextRequired) {
		t.Fatalf("CreateTask() error = %v, want %v", err, platformrepo.ErrTenantContextRequired)
	}
}

func TestScheduleRepositoryCreateRequiresTenantContext(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE execution_schedules (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			task_id TEXT NOT NULL,
			schedule_type TEXT NOT NULL,
			schedule_expr TEXT,
			scheduled_at DATETIME,
			status TEXT,
			next_run_at DATETIME,
			last_run_at DATETIME,
			enabled BOOLEAN,
			description TEXT,
			max_failures INTEGER,
			consecutive_failures INTEGER,
			pause_reason TEXT,
			target_hosts_override TEXT,
			extra_vars_override TEXT,
			secrets_source_ids TEXT,
			skip_notification BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	repo := NewScheduleRepositoryWithDB(db)
	schedule := &model.ExecutionSchedule{
		ID:           uuid.New(),
		Name:         "daily",
		TaskID:       uuid.New(),
		ScheduleType: "cron",
		Enabled:      true,
	}

	err := repo.Create(context.Background(), schedule)
	if !errors.Is(err, platformrepo.ErrTenantContextRequired) {
		t.Fatalf("Create() error = %v, want %v", err, platformrepo.ErrTenantContextRequired)
	}
}
