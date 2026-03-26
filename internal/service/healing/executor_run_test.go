package healing

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

func TestShouldFailExecuteError(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		started bool
		want    bool
	}{
		{name: "pending before start", status: model.FlowInstanceStatusPending, started: false, want: true},
		{name: "running before start", status: model.FlowInstanceStatusRunning, started: false, want: false},
		{name: "running after start", status: model.FlowInstanceStatusRunning, started: true, want: true},
		{name: "waiting approval after start", status: model.FlowInstanceStatusWaitingApproval, started: true, want: true},
		{name: "failed after start", status: model.FlowInstanceStatusFailed, started: true, want: false},
		{name: "cancelled after start", status: model.FlowInstanceStatusCancelled, started: true, want: false},
	}

	for _, tc := range cases {
		if got := shouldFailExecuteError(tc.status, tc.started); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestRestartFailedInstanceSyncsIncidentStatusToProcessing(t *testing.T) {
	db := newHealingTestDB(t)
	mustExecHealing(t, db, `
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
	mustExecHealing(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			updated_at DATETIME
		);
	`)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	instanceID := uuid.MustParse("10101010-1010-1010-1010-101010101010")
	incidentID := uuid.MustParse("20202020-2020-2020-2020-202020202020")
	tenantID := uuid.MustParse("30303030-3030-3030-3030-303030303030")
	ctx := repository.WithTenantID(context.Background(), tenantID)

	mustExecHealing(t, db, `INSERT INTO flow_instances (id, tenant_id, status) VALUES (?, ?, ?)`, instanceID.String(), tenantID.String(), model.FlowInstanceStatusFailed)
	mustExecHealing(t, db, `INSERT INTO incidents (id, tenant_id, healing_status) VALUES (?, ?, ?)`, incidentID.String(), tenantID.String(), "failed")

	executor := &FlowExecutor{instanceRepo: repository.NewFlowInstanceRepository()}
	instance := &model.FlowInstance{
		ID:         instanceID,
		IncidentID: &incidentID,
		Status:     model.FlowInstanceStatusFailed,
	}
	if err := executor.restartFailedInstance(ctx, instance); err != nil {
		t.Fatalf("restartFailedInstance(): %v", err)
	}

	type row struct {
		Status string
	}
	var flowRow row
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&flowRow).Error; err != nil {
		t.Fatalf("read flow instance: %v", err)
	}
	if flowRow.Status != model.FlowInstanceStatusRunning {
		t.Fatalf("flow status = %s, want %s", flowRow.Status, model.FlowInstanceStatusRunning)
	}

	var incidentRow row
	if err := db.Table("incidents").Select("healing_status as status").Where("id = ?", incidentID.String()).Scan(&incidentRow).Error; err != nil {
		t.Fatalf("read incident: %v", err)
	}
	if incidentRow.Status != "processing" {
		t.Fatalf("incident healing_status = %s, want processing", incidentRow.Status)
	}
}
