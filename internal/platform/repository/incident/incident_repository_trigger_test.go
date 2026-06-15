package incident

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestListTriggerRecordsIncludesTriggeredAndDismissedOnly(t *testing.T) {
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
	tenantID := uuid.New()
	otherTenantID := uuid.New()
	ruleID := uuid.New()
	flowInstanceID := uuid.New()
	now := time.Now().UTC()

	insertIncident := func(id uuid.UUID, tenant uuid.UUID, externalID, status string, scanned bool, matchedRuleID *uuid.UUID, healingFlowInstanceID *uuid.UUID, updatedAt time.Time) {
		t.Helper()
		mustExec(t, db, `
			INSERT INTO incidents (
				id, tenant_id, external_id, title, raw_data, healing_status,
				scanned, matched_rule_id, healing_flow_instance_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			id.String(),
			tenant.String(),
			externalID,
			externalID+" title",
			`{}`,
			status,
			scanned,
			nullableUUID(matchedRuleID),
			nullableUUID(healingFlowInstanceID),
			updatedAt.Add(-time.Hour),
			updatedAt,
		)
	}

	triggeredID := uuid.New()
	dismissedID := uuid.New()
	pendingID := uuid.New()
	unmatchedID := uuid.New()
	otherTenantRecordID := uuid.New()

	insertIncident(triggeredID, tenantID, "triggered", "processing", true, &ruleID, &flowInstanceID, now)
	insertIncident(dismissedID, tenantID, "dismissed", "dismissed", true, &ruleID, nil, now.Add(-time.Minute))
	insertIncident(pendingID, tenantID, "pending", "pending", true, &ruleID, nil, now.Add(-2*time.Minute))
	insertIncident(unmatchedID, tenantID, "unmatched", "dismissed", true, nil, nil, now.Add(-3*time.Minute))
	insertIncident(otherTenantRecordID, otherTenantID, "other-tenant", "dismissed", true, &ruleID, nil, now.Add(-4*time.Minute))

	records, total, err := repo.ListTriggerRecords(WithTenantID(context.Background(), tenantID), 1, 20, "", "", "", "")
	if err != nil {
		t.Fatalf("ListTriggerRecords(): %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].ID != triggeredID {
		t.Fatalf("records[0].ID = %s, want triggered %s", records[0].ID, triggeredID)
	}
	if records[1].ID != dismissedID {
		t.Fatalf("records[1].ID = %s, want dismissed %s", records[1].ID, dismissedID)
	}
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}
