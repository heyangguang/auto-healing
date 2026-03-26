package repository

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestFlowInstanceRepositoryCreateWithIncidentSyncPersistsScanState(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT NOT NULL,
			rule_id TEXT,
			incident_id TEXT,
			status TEXT NOT NULL,
			current_node_id TEXT,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			context TEXT,
			node_states TEXT,
			flow_name TEXT,
			flow_nodes TEXT,
			flow_edges TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			matched_rule_id TEXT,
			healing_flow_instance_id TEXT,
			scanned BOOLEAN,
			updated_at DATETIME
		);
	`)

	repo := &FlowInstanceRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	incidentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ruleID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	instanceID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	ctx := WithTenantID(context.Background(), tenantID)

	mustExec(t, db, `INSERT INTO incidents (id, tenant_id, healing_status, scanned) VALUES (?, ?, ?, ?)`, incidentID.String(), tenantID.String(), "pending", false)

	instance := &model.FlowInstance{
		ID:         instanceID,
		FlowID:     uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Status:     model.FlowInstanceStatusPending,
		Context:    model.JSON{},
		NodeStates: model.JSON{},
	}
	scanned := true
	if err := repo.CreateWithIncidentSync(ctx, instance, IncidentSyncOptions{
		IncidentID:     incidentID,
		HealingStatus:  "processing",
		MatchedRuleID:  &ruleID,
		FlowInstanceID: &instance.ID,
		Scanned:        &scanned,
	}); err != nil {
		t.Fatalf("CreateWithIncidentSync(): %v", err)
	}

	type incidentRow struct {
		HealingStatus  string
		MatchedRuleID  string
		FlowInstanceID string
		Scanned        bool
	}
	var row incidentRow
	if err := db.Table("incidents").Select("healing_status, matched_rule_id, healing_flow_instance_id as flow_instance_id, scanned").Where("id = ?", incidentID.String()).Scan(&row).Error; err != nil {
		t.Fatalf("read incident: %v", err)
	}
	if row.HealingStatus != "processing" {
		t.Fatalf("healing_status = %s, want processing", row.HealingStatus)
	}
	if row.MatchedRuleID != ruleID.String() {
		t.Fatalf("matched_rule_id = %s, want %s", row.MatchedRuleID, ruleID)
	}
	if row.FlowInstanceID != instanceID.String() {
		t.Fatalf("healing_flow_instance_id = %s, want %s", row.FlowInstanceID, instanceID)
	}
	if !row.Scanned {
		t.Fatal("scanned = false, want true")
	}
}

func TestFlowInstanceRepositoryUpdateStatusWithIncidentSyncRollsBackWhenIncidentMissing(t *testing.T) {
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
	tenantID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	instanceID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	ctx := WithTenantID(context.Background(), tenantID)

	mustExec(t, db, `INSERT INTO flow_instances (id, tenant_id, status) VALUES (?, ?, ?)`, instanceID.String(), tenantID.String(), model.FlowInstanceStatusRunning)

	updated, err := repo.UpdateStatusWithIncidentSync(ctx, instanceID, []string{model.FlowInstanceStatusRunning}, model.FlowInstanceStatusFailed, "boom", &IncidentSyncOptions{
		IncidentID:    uuid.MustParse("88888888-8888-8888-8888-888888888888"),
		HealingStatus: "failed",
	})
	if err == nil {
		t.Fatal("UpdateStatusWithIncidentSync() should fail when incident is missing")
	}
	if updated {
		t.Fatal("UpdateStatusWithIncidentSync() should not report updated on rollback")
	}

	var status string
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read flow status: %v", err)
	}
	if status != model.FlowInstanceStatusRunning {
		t.Fatalf("status after rollback = %s, want %s", status, model.FlowInstanceStatusRunning)
	}
}
