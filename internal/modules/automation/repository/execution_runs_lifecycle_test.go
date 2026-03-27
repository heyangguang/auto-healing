package repository

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestExecutionRepositoryCancelDoesNotOverwriteTerminalRun(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE execution_runs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT NOT NULL,
			completed_at DATETIME
		);
	`)

	repo := &ExecutionRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	runID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	ctx := WithTenantID(context.Background(), tenantID)

	mustExec(t, db, `INSERT INTO execution_runs (id, tenant_id, status) VALUES (?, ?, ?)`, runID.String(), tenantID.String(), "success")

	updated, err := repo.UpdateRunStatus(ctx, runID, "cancelled")
	if err != nil {
		t.Fatalf("UpdateRunStatus(cancelled): %v", err)
	}
	if updated {
		t.Fatal("UpdateRunStatus(cancelled) should not overwrite terminal run")
	}

	var status string
	if err := db.Table("execution_runs").Select("status").Where("id = ?", runID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read execution run status: %v", err)
	}
	if status != "success" {
		t.Fatalf("status after cancel attempt = %s, want success", status)
	}
}

func TestExecutionRepositoryCancelOnlyUpdatesActiveStatuses(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE execution_runs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT NOT NULL,
			completed_at DATETIME
		);
	`)

	repo := &ExecutionRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	runID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	ctx := WithTenantID(context.Background(), tenantID)

	mustExec(t, db, `INSERT INTO execution_runs (id, tenant_id, status) VALUES (?, ?, ?)`, runID.String(), tenantID.String(), model.FlowInstanceStatusRunning)

	updated, err := repo.UpdateRunStatus(ctx, runID, "cancelled")
	if err != nil {
		t.Fatalf("UpdateRunStatus(cancelled): %v", err)
	}
	if !updated {
		t.Fatal("UpdateRunStatus(cancelled) should update active run")
	}
}
