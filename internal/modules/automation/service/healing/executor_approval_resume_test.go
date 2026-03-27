package healing

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

func TestResumeAfterApprovalFailsInstanceWhenCurrentNodeIsNotApproval(t *testing.T) {
	db := newHealingTestDB(t)
	mustExecHealing(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT,
			status TEXT NOT NULL,
			current_node_id TEXT,
			flow_nodes TEXT,
			flow_edges TEXT,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			updated_at DATETIME,
			node_states TEXT
		);
	`)
	mustExecHealing(t, db, `CREATE TABLE healing_rules (id TEXT PRIMARY KEY NOT NULL);`)
	mustExecHealing(t, db, `CREATE TABLE incidents (id TEXT PRIMARY KEY NOT NULL);`)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	executor := &FlowExecutor{instanceRepo: repository.NewFlowInstanceRepository()}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	instanceID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	flowID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	ctx := repository.WithTenantID(context.Background(), tenantID)

	nodes := `[{"id":"start-node","type":"start","position":{"x":0,"y":0},"data":{}}]`
	mustExecHealing(t, db, `INSERT INTO flow_instances (id, tenant_id, flow_id, status, current_node_id, flow_nodes, flow_edges, node_states) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		instanceID.String(), tenantID.String(), flowID.String(), model.FlowInstanceStatusWaitingApproval, "start-node", nodes, "[]", "{}")

	err := executor.ResumeAfterApproval(ctx, instanceID, true)
	if err == nil {
		t.Fatal("ResumeAfterApproval() should fail when current node is not approval")
	}

	type instanceRow struct {
		Status       string
		ErrorMessage string
	}
	var row instanceRow
	if err := db.Table("flow_instances").Select("status, error_message").Where("id = ?", instanceID.String()).Scan(&row).Error; err != nil {
		t.Fatalf("read flow instance: %v", err)
	}
	if row.Status != model.FlowInstanceStatusFailed {
		t.Fatalf("status after resume failure = %s, want %s", row.Status, model.FlowInstanceStatusFailed)
	}
	if row.ErrorMessage == "" {
		t.Fatal("error_message should be persisted after resume failure")
	}
}
