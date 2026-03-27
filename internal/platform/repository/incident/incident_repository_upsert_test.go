package incident

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestIncidentUpsertPreservesTenantAndHealingMetadata(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			plugin_id TEXT,
			source_plugin_name TEXT,
			external_id TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			severity TEXT,
			priority TEXT,
			status TEXT,
			category TEXT,
			affected_ci TEXT,
			affected_service TEXT,
			assignee TEXT,
			reporter TEXT,
			raw_data TEXT NOT NULL,
			healing_status TEXT,
			workflow_instance_id TEXT,
			scanned BOOLEAN,
			matched_rule_id TEXT,
			healing_flow_instance_id TEXT,
			source_created_at DATETIME,
			source_updated_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)

	repo := &IncidentRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	incidentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	workflowID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	ruleID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	flowInstanceID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	ctx := WithTenantID(context.Background(), tenantID)

	mustExec(t, db, `
		INSERT INTO incidents (
			id, tenant_id, plugin_id, source_plugin_name, external_id, title, raw_data,
			healing_status, workflow_instance_id, scanned, matched_rule_id, healing_flow_instance_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		incidentID.String(),
		tenantID.String(),
		pluginID.String(),
		"plugin-a",
		"ext-1",
		"old-title",
		`{"old":"value"}`,
		"processing",
		workflowID.String(),
		true,
		ruleID.String(),
		flowInstanceID.String(),
	)

	incoming := &model.Incident{
		PluginID:         &pluginID,
		SourcePluginName: "plugin-a",
		ExternalID:       "ext-1",
		Title:            "new-title",
		RawData:          model.JSON{"new": "value"},
		HealingStatus:    "pending",
	}
	isNew, err := repo.UpsertByExternalID(ctx, incoming)
	if err != nil {
		t.Fatalf("UpsertByExternalID(): %v", err)
	}
	if isNew {
		t.Fatal("UpsertByExternalID() should update existing row")
	}
	if incoming.TenantID == nil || *incoming.TenantID != tenantID {
		t.Fatalf("incoming tenant_id = %v, want %s", incoming.TenantID, tenantID)
	}
	if incoming.HealingStatus != "processing" {
		t.Fatalf("incoming healing_status = %s, want processing", incoming.HealingStatus)
	}

	var stored model.Incident
	if err := db.Where("id = ?", incidentID.String()).First(&stored).Error; err != nil {
		t.Fatalf("query incident: %v", err)
	}
	if stored.TenantID == nil || *stored.TenantID != tenantID {
		t.Fatalf("stored tenant_id = %v, want %s", stored.TenantID, tenantID)
	}
	if stored.Title != "new-title" {
		t.Fatalf("stored title = %s, want new-title", stored.Title)
	}
	if stored.HealingStatus != "processing" {
		t.Fatalf("stored healing_status = %s, want processing", stored.HealingStatus)
	}
	if stored.WorkflowInstanceID == nil || *stored.WorkflowInstanceID != workflowID {
		t.Fatalf("stored workflow_instance_id = %v, want %s", stored.WorkflowInstanceID, workflowID)
	}
	if !stored.Scanned {
		t.Fatal("stored scanned should stay true")
	}
	if stored.MatchedRuleID == nil || *stored.MatchedRuleID != ruleID {
		t.Fatalf("stored matched_rule_id = %v, want %s", stored.MatchedRuleID, ruleID)
	}
	if stored.HealingFlowInstanceID == nil || *stored.HealingFlowInstanceID != flowInstanceID {
		t.Fatalf("stored healing_flow_instance_id = %v, want %s", stored.HealingFlowInstanceID, flowInstanceID)
	}
}
