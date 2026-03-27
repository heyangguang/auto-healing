package repository

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
)

func TestFlowInstanceCreateWithIncidentSyncRollsBackWhenIncidentMissing(t *testing.T) {
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
			scanned BOOLEAN,
			matched_rule_id TEXT,
			healing_flow_instance_id TEXT,
			updated_at DATETIME
		);
	`)

	repo := &FlowInstanceRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ctx := WithTenantID(context.Background(), tenantID)
	instance := &model.FlowInstance{
		ID:         uuid.MustParse("abababab-abab-abab-abab-abababababab"),
		FlowID:     uuid.MustParse("cdcdcdcd-cdcd-cdcd-cdcd-cdcdcdcdcdcd"),
		Status:     model.FlowInstanceStatusPending,
		Context:    model.JSON{},
		NodeStates: model.JSON{},
	}

	scanned := true
	err := repo.CreateWithIncidentSync(ctx, instance, IncidentSyncOptions{
		IncidentID:    uuid.MustParse("12121212-1212-1212-1212-121212121212"),
		HealingStatus: "processing",
		Scanned:       &scanned,
	})
	if err == nil {
		t.Fatal("CreateWithIncidentSync() should fail when incident is missing")
	}

	var count int64
	if err := db.Table("flow_instances").Count(&count).Error; err != nil {
		t.Fatalf("count flow_instances: %v", err)
	}
	if count != 0 {
		t.Fatalf("flow_instances rows after rollback = %d, want 0", count)
	}
}

func TestFlowInstanceCreateWithIncidentSyncPersistsIncidentLinkage(t *testing.T) {
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
			scanned BOOLEAN,
			matched_rule_id TEXT,
			healing_flow_instance_id TEXT,
			updated_at DATETIME
		);
	`)

	repo := &FlowInstanceRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	incidentID := uuid.MustParse("13131313-1313-1313-1313-131313131313")
	ruleID := uuid.MustParse("14141414-1414-1414-1414-141414141414")
	instanceID := uuid.MustParse("15151515-1515-1515-1515-151515151515")
	ctx := WithTenantID(context.Background(), tenantID)
	mustExec(t, db, `INSERT INTO incidents (id, tenant_id, healing_status, scanned) VALUES (?, ?, ?, ?)`, incidentID.String(), tenantID.String(), "pending", false)

	instance := &model.FlowInstance{
		ID:         instanceID,
		FlowID:     uuid.MustParse("16161616-1616-1616-1616-161616161616"),
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
		HealingStatus         string
		Scanned               bool
		MatchedRuleID         string
		HealingFlowInstanceID string
	}
	var row incidentRow
	if err := db.Table("incidents").Select("healing_status, scanned, matched_rule_id, healing_flow_instance_id").Where("id = ?", incidentID.String()).Scan(&row).Error; err != nil {
		t.Fatalf("read incident row: %v", err)
	}
	if row.HealingStatus != "processing" || !row.Scanned {
		t.Fatalf("incident sync mismatch: status=%s scanned=%v", row.HealingStatus, row.Scanned)
	}
	if row.MatchedRuleID != ruleID.String() {
		t.Fatalf("matched_rule_id = %s, want %s", row.MatchedRuleID, ruleID)
	}
	if row.HealingFlowInstanceID != instanceID.String() {
		t.Fatalf("healing_flow_instance_id = %s, want %s", row.HealingFlowInstanceID, instanceID)
	}
}
