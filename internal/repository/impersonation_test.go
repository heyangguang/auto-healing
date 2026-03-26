package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func createImpersonationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE impersonation_requests (
			id TEXT PRIMARY KEY NOT NULL,
			requester_id TEXT NOT NULL,
			requester_name TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			tenant_name TEXT NOT NULL,
			reason TEXT,
			duration_minutes INTEGER NOT NULL,
			status TEXT NOT NULL,
			approved_by TEXT,
			approved_at DATETIME,
			session_started_at DATETIME,
			session_expires_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
}

func TestImpersonationUpdateStatusRejectsNonPendingRequest(t *testing.T) {
	db := newSQLiteTestDB(t)
	createImpersonationSchema(t, db)

	repo := &ImpersonationRepository{db: db}
	requestID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	now := time.Now().UTC()
	mustExec(t, db, `
		INSERT INTO impersonation_requests (id, requester_id, requester_name, tenant_id, tenant_name, duration_minutes, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, requestID.String(), uuid.NewString(), "requester", uuid.NewString(), "tenant", 30, model.ImpersonationStatusApproved, now, now)

	err := repo.UpdateStatus(context.Background(), requestID, model.ImpersonationStatusRejected, nil)
	if !errors.Is(err, ErrImpersonationRequestNotPending) {
		t.Fatalf("UpdateStatus() error = %v, want ErrImpersonationRequestNotPending", err)
	}
}

func TestImpersonationGetOpenRequestIncludesApprovedRequest(t *testing.T) {
	db := newSQLiteTestDB(t)
	createImpersonationSchema(t, db)

	repo := &ImpersonationRepository{db: db}
	requesterID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tenantID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	requestID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	now := time.Now().UTC()
	mustExec(t, db, `
		INSERT INTO impersonation_requests (id, requester_id, requester_name, tenant_id, tenant_name, duration_minutes, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, requestID.String(), requesterID.String(), "requester", tenantID.String(), "tenant", 30, model.ImpersonationStatusApproved, now, now)

	req, err := repo.GetOpenRequest(context.Background(), requesterID, tenantID)
	if err != nil {
		t.Fatalf("GetOpenRequest() error = %v", err)
	}
	if req == nil || req.ID != requestID {
		t.Fatalf("GetOpenRequest() = %#v, want request %s", req, requestID)
	}
}
