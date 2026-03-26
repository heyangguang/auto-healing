package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func createBlacklistExemptionSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE blacklist_exemptions (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			task_id TEXT NOT NULL,
			task_name TEXT,
			rule_id TEXT NOT NULL,
			rule_name TEXT,
			rule_severity TEXT,
			rule_pattern TEXT,
			reason TEXT NOT NULL,
			requested_by TEXT NOT NULL,
			requester_name TEXT,
			status TEXT NOT NULL,
			approved_by TEXT,
			approver_name TEXT,
			approved_at DATETIME,
			reject_reason TEXT,
			validity_days INTEGER NOT NULL DEFAULT 30,
			expires_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
}

func TestBlacklistExemptionListTreatsExpiredApprovedAsExpired(t *testing.T) {
	db := newSQLiteTestDB(t)
	createBlacklistExemptionSchema(t, db)

	repo := &BlacklistExemptionRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	now := time.Now().UTC()

	activeID := uuid.MustParse("22222222-2222-2222-2222-222222222221")
	expiredID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	mustExec(t, db, `
		INSERT INTO blacklist_exemptions (id, tenant_id, task_id, rule_id, reason, requested_by, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, activeID.String(), tenantID.String(), uuid.NewString(), uuid.NewString(), "active", uuid.NewString(), "approved", now.Add(time.Hour), now, now)
	mustExec(t, db, `
		INSERT INTO blacklist_exemptions (id, tenant_id, task_id, rule_id, reason, requested_by, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, expiredID.String(), tenantID.String(), uuid.NewString(), uuid.NewString(), "expired", uuid.NewString(), "approved", now.Add(-time.Hour), now, now)

	ctx := WithTenantID(context.Background(), tenantID)

	approved, total, err := repo.List(ctx, ExemptionListOptions{Page: 1, PageSize: 10, Status: "approved"})
	if err != nil {
		t.Fatalf("List approved: %v", err)
	}
	if total != 1 || len(approved) != 1 || approved[0].ID != activeID {
		t.Fatalf("approved list = %#v total=%d, want only active approved exemption", approved, total)
	}

	expired, total, err := repo.List(ctx, ExemptionListOptions{Page: 1, PageSize: 10, Status: "expired"})
	if err != nil {
		t.Fatalf("List expired: %v", err)
	}
	if total != 1 || len(expired) != 1 || expired[0].ID != expiredID {
		t.Fatalf("expired list = %#v total=%d, want only overdue exemption", expired, total)
	}
	if expired[0].Status != "expired" {
		t.Fatalf("expired item status = %q, want expired", expired[0].Status)
	}
}

func TestBlacklistExemptionGetNormalizesExpiredStatus(t *testing.T) {
	db := newSQLiteTestDB(t)
	createBlacklistExemptionSchema(t, db)

	repo := &BlacklistExemptionRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	itemID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	now := time.Now().UTC()
	mustExec(t, db, `
		INSERT INTO blacklist_exemptions (id, tenant_id, task_id, rule_id, reason, requested_by, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, itemID.String(), tenantID.String(), uuid.NewString(), uuid.NewString(), "expired", uuid.NewString(), "approved", now.Add(-time.Minute), now, now)

	item, err := repo.Get(WithTenantID(context.Background(), tenantID), itemID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if item.Status != "expired" {
		t.Fatalf("Get status = %q, want expired", item.Status)
	}
}

func TestBlacklistExemptionGetApprovedByTaskIDSkipsExpired(t *testing.T) {
	db := newSQLiteTestDB(t)
	createBlacklistExemptionSchema(t, db)

	repo := &BlacklistExemptionRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	taskID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	now := time.Now().UTC()
	mustExec(t, db, `
		INSERT INTO blacklist_exemptions (id, tenant_id, task_id, rule_id, reason, requested_by, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), tenantID.String(), taskID.String(), uuid.NewString(), "active", uuid.NewString(), "approved", now.Add(time.Hour), now, now)
	mustExec(t, db, `
		INSERT INTO blacklist_exemptions (id, tenant_id, task_id, rule_id, reason, requested_by, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), tenantID.String(), taskID.String(), uuid.NewString(), "expired", uuid.NewString(), "approved", now.Add(-time.Hour), now, now)

	items, err := repo.GetApprovedByTaskID(WithTenantID(context.Background(), tenantID), taskID)
	if err != nil {
		t.Fatalf("GetApprovedByTaskID: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("GetApprovedByTaskID len = %d, want 1", len(items))
	}
}

func TestBlacklistExemptionUpdateStatusRejectsNonPendingRequest(t *testing.T) {
	db := newSQLiteTestDB(t)
	createBlacklistExemptionSchema(t, db)

	repo := &BlacklistExemptionRepository{db: db}
	tenantID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	itemID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	now := time.Now().UTC()
	mustExec(t, db, `
		INSERT INTO blacklist_exemptions (id, tenant_id, task_id, rule_id, reason, requested_by, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, itemID.String(), tenantID.String(), uuid.NewString(), uuid.NewString(), "done", uuid.NewString(), "approved", now, now)

	err := repo.UpdateStatus(WithTenantID(context.Background(), tenantID), itemID, map[string]interface{}{
		"status":     "rejected",
		"updated_at": now,
	})
	if !errors.Is(err, ErrBlacklistExemptionNotPending) {
		t.Fatalf("UpdateStatus() error = %v, want ErrBlacklistExemptionNotPending", err)
	}
}
