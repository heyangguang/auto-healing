package repository

import (
	"testing"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
)

func TestLoadExecutionTaskNamesUsesScopedDB(t *testing.T) {
	db := newSQLiteTestDB(t)
	mustExec(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL
		);
	`)

	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	taskA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	taskB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	mustExec(t, db, `INSERT INTO execution_tasks (id, tenant_id, name) VALUES (?, ?, ?)`, taskA.String(), tenantA.String(), "tenant-a-task")
	mustExec(t, db, `INSERT INTO execution_tasks (id, tenant_id, name) VALUES (?, ?, ?)`, taskB.String(), tenantB.String(), "tenant-b-task")

	repo := &SearchRepository{db: db}
	taskNames, err := repo.loadExecutionTaskNames(db.Where("tenant_id = ?", tenantA), []model.ExecutionRun{
		{TaskID: taskA},
		{TaskID: taskB},
	})
	if err != nil {
		t.Fatalf("loadExecutionTaskNames() error = %v", err)
	}
	if taskNames[taskA] != "tenant-a-task" {
		t.Fatalf("taskNames[taskA] = %q, want tenant-a-task", taskNames[taskA])
	}
	if _, exists := taskNames[taskB]; exists {
		t.Fatalf("taskNames leaked cross-tenant task: %#v", taskNames)
	}
}
