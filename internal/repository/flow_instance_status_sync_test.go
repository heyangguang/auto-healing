package repository

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestFlowInstanceUpdateStatusWithIncidentSyncIsAtomic(t *testing.T) {
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
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			updated_at DATETIME
		);
	`)

	repo := &FlowInstanceRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	instanceID := uuid.MustParse("17171717-1717-1717-1717-171717171717")
	ctx := WithTenantID(context.Background(), tenantID)
	mustExec(t, db, `INSERT INTO flow_instances (id, tenant_id, status) VALUES (?, ?, ?)`, instanceID.String(), tenantID.String(), model.FlowInstanceStatusRunning)

	updated, err := repo.UpdateStatusWithIncidentSync(ctx, instanceID, []string{model.FlowInstanceStatusRunning}, model.FlowInstanceStatusFailed, "boom", &IncidentSyncOptions{
		IncidentID:    uuid.MustParse("18181818-1818-1818-1818-181818181818"),
		HealingStatus: "failed",
	})
	if err == nil {
		t.Fatal("UpdateStatusWithIncidentSync() should fail when incident is missing")
	}
	if updated {
		t.Fatal("UpdateStatusWithIncidentSync() should not report updated after rollback")
	}

	var status string
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read flow instance status: %v", err)
	}
	if status != model.FlowInstanceStatusRunning {
		t.Fatalf("status after rollback = %s, want %s", status, model.FlowInstanceStatusRunning)
	}
}

func TestFlowInstanceUpdateStatusWithIncidentSyncPersistsSuccessPath(t *testing.T) {
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
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			updated_at DATETIME
		);
	`)

	repo := &FlowInstanceRepository{db: db}
	tenantID := uuid.MustParse("19191919-1919-1919-1919-191919191919")
	instanceID := uuid.MustParse("20202020-2020-2020-2020-202020202020")
	incidentID := uuid.MustParse("21212121-2121-2121-2121-212121212121")
	ctx := WithTenantID(context.Background(), tenantID)
	mustExec(t, db, `INSERT INTO flow_instances (id, tenant_id, status) VALUES (?, ?, ?)`, instanceID.String(), tenantID.String(), model.FlowInstanceStatusFailed)
	mustExec(t, db, `INSERT INTO incidents (id, tenant_id, healing_status) VALUES (?, ?, ?)`, incidentID.String(), tenantID.String(), "failed")

	updated, err := repo.UpdateStatusWithIncidentSync(ctx, instanceID, []string{model.FlowInstanceStatusFailed}, model.FlowInstanceStatusRunning, "", &IncidentSyncOptions{
		IncidentID:    incidentID,
		HealingStatus: "processing",
	})
	if err != nil {
		t.Fatalf("UpdateStatusWithIncidentSync(): %v", err)
	}
	if !updated {
		t.Fatal("UpdateStatusWithIncidentSync() should update matching instance")
	}

	var flowStatus string
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&flowStatus).Error; err != nil {
		t.Fatalf("read flow status: %v", err)
	}
	if flowStatus != model.FlowInstanceStatusRunning {
		t.Fatalf("flow status = %s, want %s", flowStatus, model.FlowInstanceStatusRunning)
	}

	var healingStatus string
	if err := db.Table("incidents").Select("healing_status").Where("id = ?", incidentID.String()).Scan(&healingStatus).Error; err != nil {
		t.Fatalf("read incident healing_status: %v", err)
	}
	if healingStatus != "processing" {
		t.Fatalf("incident healing_status = %s, want processing", healingStatus)
	}
}
