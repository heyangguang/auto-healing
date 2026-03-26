package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

type tenantScopedTestModel struct {
	ID       uuid.UUID  `gorm:"column:id;primaryKey"`
	TenantID *uuid.UUID `gorm:"column:tenant_id"`
	Name     string     `gorm:"column:name"`
}

func (tenantScopedTestModel) TableName() string {
	return "tenant_scoped_test_models"
}

func TestTenantDBRequiresTenantContext(t *testing.T) {
	db := newStateTestDB(t)

	tx := TenantDB(db, context.Background())
	if !errors.Is(tx.Error, ErrTenantContextRequired) {
		t.Fatalf("TenantDB() error = %v, want %v", tx.Error, ErrTenantContextRequired)
	}
}

func TestTenantIDFromContextMissingReturnsNil(t *testing.T) {
	if got := TenantIDFromContext(context.Background()); got != uuid.Nil {
		t.Fatalf("TenantIDFromContext() = %v, want %v", got, uuid.Nil)
	}
}

func TestFillTenantIDRequiresTenantContext(t *testing.T) {
	var tenantID *uuid.UUID
	err := FillTenantID(context.Background(), &tenantID)
	if !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("FillTenantID() error = %v, want %v", err, ErrTenantContextRequired)
	}
}

func TestFillTenantIDUsesExplicitTenantContext(t *testing.T) {
	expected := uuid.New()
	ctx := WithTenantID(context.Background(), expected)

	var tenantID *uuid.UUID
	if err := FillTenantID(ctx, &tenantID); err != nil {
		t.Fatalf("FillTenantID() error = %v", err)
	}
	if tenantID == nil || *tenantID != expected {
		t.Fatalf("FillTenantID() tenantID = %v, want %v", tenantID, expected)
	}
}

func TestUpdateTenantScopedModelRequiresTenantContext(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE tenant_scoped_test_models (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT
		);
	`)

	tenantID := uuid.New()
	entity := &tenantScopedTestModel{ID: uuid.New(), TenantID: &tenantID, Name: "updated"}

	err := UpdateTenantScopedModel(db, context.Background(), entity.ID, entity)
	if !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("UpdateTenantScopedModel() error = %v, want %v", err, ErrTenantContextRequired)
	}
}

func TestUpdateTenantScopedModelUsesTenantScope(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE tenant_scoped_test_models (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT
		);
	`)

	tenantA := uuid.New()
	tenantB := uuid.New()
	entityID := uuid.New()
	mustExec(t, db, `INSERT INTO tenant_scoped_test_models (id, tenant_id, name) VALUES (?, ?, ?)`, entityID.String(), tenantA.String(), "before")

	ctxA := WithTenantID(context.Background(), tenantA)
	ctxB := WithTenantID(context.Background(), tenantB)
	update := &tenantScopedTestModel{ID: entityID, TenantID: &tenantA, Name: "after"}

	if err := UpdateTenantScopedModel(db, ctxB, entityID, update); err != nil {
		t.Fatalf("UpdateTenantScopedModel() wrong tenant error = %v", err)
	}

	var afterWrongTenant tenantScopedTestModel
	if err := db.WithContext(context.Background()).First(&afterWrongTenant, "id = ?", entityID).Error; err != nil {
		t.Fatalf("query after wrong tenant update error = %v", err)
	}
	if afterWrongTenant.Name != "before" {
		t.Fatalf("wrong-tenant update changed row name to %q, want %q", afterWrongTenant.Name, "before")
	}

	if err := UpdateTenantScopedModel(db, ctxA, entityID, update); err != nil {
		t.Fatalf("UpdateTenantScopedModel() correct tenant error = %v", err)
	}

	var afterCorrectTenant tenantScopedTestModel
	if err := db.WithContext(context.Background()).First(&afterCorrectTenant, "id = ?", entityID).Error; err != nil {
		t.Fatalf("query after correct tenant update error = %v", err)
	}
	if afterCorrectTenant.Name != "after" {
		t.Fatalf("correct-tenant update name = %q, want %q", afterCorrectTenant.Name, "after")
	}
}

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

	repo := NewExecutionRepository()
	task := &model.ExecutionTask{
		ID:           uuid.New(),
		Name:         "task",
		ExecutorType: "local",
	}

	err := repo.CreateTask(context.Background(), task)
	if !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("CreateTask() error = %v, want %v", err, ErrTenantContextRequired)
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

	repo := NewScheduleRepository()
	schedule := &model.ExecutionSchedule{
		ID:           uuid.New(),
		Name:         "daily",
		TaskID:       uuid.New(),
		ScheduleType: "cron",
		Enabled:      true,
	}

	err := repo.Create(context.Background(), schedule)
	if !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("Create() error = %v, want %v", err, ErrTenantContextRequired)
	}
}
