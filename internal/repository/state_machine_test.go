package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newStateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "state.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func TestFlowInstanceRepositoryStateTransitions(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			updated_at DATETIME
		);
	`)

	repo := &FlowInstanceRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	instanceID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	ctx := WithTenantID(context.Background(), tenantID)

	mustExec(t, db, `INSERT INTO flow_instances (id, tenant_id, status) VALUES (?, ?, ?)`, instanceID.String(), tenantID.String(), model.FlowInstanceStatusPending)

	started, err := repo.Start(ctx, instanceID)
	if err != nil || !started {
		t.Fatalf("Start() started=%v err=%v", started, err)
	}

	var status string
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read flow instance status: %v", err)
	}
	if status != model.FlowInstanceStatusRunning {
		t.Fatalf("status after Start() = %s, want %s", status, model.FlowInstanceStatusRunning)
	}

	entered, err := repo.EnterWaitingApproval(ctx, instanceID)
	if err != nil || !entered {
		t.Fatalf("EnterWaitingApproval() entered=%v err=%v", entered, err)
	}
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read flow instance status after waiting approval: %v", err)
	}
	if status != model.FlowInstanceStatusWaitingApproval {
		t.Fatalf("status after EnterWaitingApproval() = %s, want %s", status, model.FlowInstanceStatusWaitingApproval)
	}

	cancelled, err := repo.UpdateStatusIfCurrent(ctx, instanceID, []string{model.FlowInstanceStatusWaitingApproval}, model.FlowInstanceStatusCancelled, "cancelled")
	if err != nil || !cancelled {
		t.Fatalf("UpdateStatusIfCurrent(cancelled) updated=%v err=%v", cancelled, err)
	}
	failed, err := repo.UpdateStatusIfCurrent(ctx, instanceID, []string{model.FlowInstanceStatusWaitingApproval}, model.FlowInstanceStatusFailed, "timeout")
	if err != nil {
		t.Fatalf("UpdateStatusIfCurrent(failed) err=%v", err)
	}
	if failed {
		t.Fatalf("UpdateStatusIfCurrent should not overwrite cancelled instance")
	}
}

func TestExecutionRepositoryCancelledRunIsNotOverwritten(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE execution_runs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT NOT NULL,
			task_id TEXT,
			exit_code INTEGER,
			stdout TEXT,
			stderr TEXT,
			stats TEXT,
			started_at DATETIME,
			completed_at DATETIME
		);
	`)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	repo := NewExecutionRepository()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	runID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	ctx := WithTenantID(context.Background(), tenantID)

	mustExec(t, db, `INSERT INTO execution_runs (id, tenant_id, status) VALUES (?, ?, ?)`, runID.String(), tenantID.String(), "pending")

	started, err := repo.UpdateRunStarted(ctx, runID)
	if err != nil || !started {
		t.Fatalf("UpdateRunStarted() started=%v err=%v", started, err)
	}
	cancelled, err := repo.UpdateRunStatus(ctx, runID, "cancelled")
	if err != nil {
		t.Fatalf("UpdateRunStatus(cancelled): %v", err)
	}
	if !cancelled {
		t.Fatal("UpdateRunStatus(cancelled) should update running run")
	}
	if err := repo.UpdateRunResult(ctx, runID, 0, "ok", "", nil); err != nil {
		t.Fatalf("UpdateRunResult(): %v", err)
	}

	var status string
	if err := db.Table("execution_runs").Select("status").Where("id = ?", runID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read execution run status: %v", err)
	}
	if status != "cancelled" {
		t.Fatalf("status after UpdateRunResult() = %s, want cancelled", status)
	}
}

func TestApprovalTaskCreateAndCancelLifecycle(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE approval_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_instance_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			initiated_by TEXT,
			status TEXT NOT NULL,
			timeout_at DATETIME,
			approvers TEXT,
			approver_roles TEXT,
			decided_by TEXT,
			decision_comment TEXT,
			decided_at DATETIME,
			notification_sent BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)

	repo := &ApprovalTaskRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	instanceID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	ctx := WithTenantID(context.Background(), tenantID)

	mustExec(t, db, `INSERT INTO flow_instances (id, tenant_id, status) VALUES (?, ?, ?)`, instanceID.String(), tenantID.String(), model.FlowInstanceStatusRunning)

	task := &model.ApprovalTask{
		ID:             uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		FlowInstanceID: instanceID,
		NodeID:         "approval_node",
		Status:         model.ApprovalTaskStatusPending,
	}
	if err := repo.CreateAndEnterWaiting(ctx, task); err != nil {
		t.Fatalf("CreateAndEnterWaiting(): %v", err)
	}

	var status string
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read flow instance status: %v", err)
	}
	if status != model.FlowInstanceStatusWaitingApproval {
		t.Fatalf("flow status after CreateAndEnterWaiting = %s, want %s", status, model.FlowInstanceStatusWaitingApproval)
	}

	cancelled, err := repo.CancelPendingByFlowInstance(ctx, instanceID, "cancelled")
	if err != nil {
		t.Fatalf("CancelPendingByFlowInstance(): %v", err)
	}
	if cancelled != 1 {
		t.Fatalf("CancelPendingByFlowInstance() rows = %d, want 1", cancelled)
	}

	var taskStatus string
	if err := db.Table("approval_tasks").Select("status").Where("id = ?", task.ID.String()).Scan(&taskStatus).Error; err != nil {
		t.Fatalf("read approval task status: %v", err)
	}
	if taskStatus != model.ApprovalTaskStatusCancelled {
		t.Fatalf("approval task status after cancel = %s, want %s", taskStatus, model.ApprovalTaskStatusCancelled)
	}
}

func TestApprovalTaskCreateAndEnterWaitingRollsBackWhenInstanceStateMismatch(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT NOT NULL,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE approval_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_instance_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			initiated_by TEXT,
			status TEXT NOT NULL,
			timeout_at DATETIME,
			approvers TEXT,
			approver_roles TEXT,
			decided_by TEXT,
			decision_comment TEXT,
			decided_at DATETIME,
			notification_sent BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)

	repo := &ApprovalTaskRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	instanceID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	ctx := WithTenantID(context.Background(), tenantID)

	mustExec(t, db, `INSERT INTO flow_instances (id, tenant_id, status) VALUES (?, ?, ?)`, instanceID.String(), tenantID.String(), model.FlowInstanceStatusCancelled)

	task := &model.ApprovalTask{
		ID:             uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
		FlowInstanceID: instanceID,
		NodeID:         "approval_node",
		Status:         model.ApprovalTaskStatusPending,
	}
	if err := repo.CreateAndEnterWaiting(ctx, task); err != ErrFlowInstanceStateConflict {
		t.Fatalf("CreateAndEnterWaiting() error = %v, want %v", err, ErrFlowInstanceStateConflict)
	}

	var count int64
	if err := db.Table("approval_tasks").Count(&count).Error; err != nil {
		t.Fatalf("count approval tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("approval task rows after rollback = %d, want 0", count)
	}
}
