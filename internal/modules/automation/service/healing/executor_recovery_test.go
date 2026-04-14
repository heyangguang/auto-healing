package healing

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestRecoverInstanceResumesCompletedExecutionNode(t *testing.T) {
	db := newHealingTestDB(t)
	mustExecHealing(t, db, `
		CREATE TABLE healing_flows (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			description TEXT,
			nodes TEXT,
			edges TEXT,
			is_active BOOLEAN,
			auto_close_source_incident BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT NOT NULL,
			rule_id TEXT,
			incident_id TEXT,
			status TEXT NOT NULL,
			current_node_id TEXT,
			context TEXT,
			node_states TEXT,
			flow_name TEXT,
			flow_nodes TEXT,
			flow_edges TEXT,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE flow_execution_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_instance_id TEXT,
			node_id TEXT,
			node_type TEXT,
			level TEXT,
			message TEXT,
			details TEXT,
			created_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE flow_recovery_attempts (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_instance_id TEXT,
			trigger_source TEXT,
			current_node_id TEXT,
			current_node_type TEXT,
			detect_reason TEXT,
			recovery_action TEXT,
			status TEXT,
			details TEXT,
			error_message TEXT,
			started_at DATETIME,
			finished_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)

	tenantID := uuid.New()
	instanceID := uuid.New()
	flowID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	nodes := `[{"id":"execute_node","type":"execution","name":"执行任务","config":{}},{"id":"end_node","type":"end","name":"结束","config":{}}]`
	edges := `[{"source":"execute_node","sourceHandle":"success","target":"end_node"}]`
	contextJSON := `{"execution_result":{"status":"completed","message":"执行成功","run":{"run_id":"` + uuid.NewString() + `","status":"success","exit_code":0,"stats":{"ok":1}}}}`
	nodeStates := `{"execute_node":{"status":"completed","message":"执行成功","run":{"run_id":"` + uuid.NewString() + `","status":"success","exit_code":0,"stats":{"ok":1}}}}`
	mustExecHealing(t, db, `
		INSERT INTO healing_flows (id, tenant_id, name, nodes, edges, is_active, auto_close_source_incident)
		VALUES (?, ?, 'flow', ?, ?, 1, 0)
	`, flowID.String(), tenantID.String(), nodes, edges)
	mustExecHealing(t, db, `
		INSERT INTO flow_instances (id, tenant_id, flow_id, status, current_node_id, context, node_states, flow_name, flow_nodes, flow_edges)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, instanceID.String(), tenantID.String(), flowID.String(), model.FlowInstanceStatusRunning, "execute_node", contextJSON, nodeStates, "flow", nodes, edges)

	executor := &FlowExecutor{
		instanceRepo:   automationrepo.NewFlowInstanceRepositoryWithDB(db),
		flowRepo:       automationrepo.NewHealingFlowRepositoryWithDB(db),
		flowLogRepo:    automationrepo.NewFlowLogRepositoryWithDB(db),
		recoveryRepo:   automationrepo.NewFlowRecoveryAttemptRepositoryWithDB(db),
		incidentCloser: &stubIncidentCloser{},
		eventBus:       GetEventBus(),
	}

	attempt, err := executor.RecoverInstance(ctx, instanceID, model.FlowRecoveryTriggerManual)
	if err != nil {
		t.Fatalf("RecoverInstance(): %v", err)
	}
	if attempt.Status != model.FlowRecoveryStatusSuccess {
		t.Fatalf("attempt status = %s, want success", attempt.Status)
	}

	var row struct {
		Status        string
		CurrentNodeID string
	}
	if err := db.Table("flow_instances").Select("status,current_node_id").Where("id = ?", instanceID.String()).Scan(&row).Error; err != nil {
		t.Fatalf("read instance: %v", err)
	}
	if row.Status != model.FlowInstanceStatusCompleted {
		t.Fatalf("instance status = %s, want completed", row.Status)
	}
	if row.CurrentNodeID != "end_node" {
		t.Fatalf("current_node_id = %s, want end_node", row.CurrentNodeID)
	}
}

func TestRecoverInstanceResumesApprovedApprovalNode(t *testing.T) {
	db := newHealingTestDB(t)
	mustExecHealing(t, db, `
		CREATE TABLE healing_flows (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			description TEXT,
			nodes TEXT,
			edges TEXT,
			is_active BOOLEAN,
			auto_close_source_incident BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT NOT NULL,
			rule_id TEXT,
			incident_id TEXT,
			status TEXT NOT NULL,
			current_node_id TEXT,
			context TEXT,
			node_states TEXT,
			flow_name TEXT,
			flow_nodes TEXT,
			flow_edges TEXT,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE flow_execution_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_instance_id TEXT,
			node_id TEXT,
			node_type TEXT,
			level TEXT,
			message TEXT,
			details TEXT,
			created_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE flow_recovery_attempts (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_instance_id TEXT,
			trigger_source TEXT,
			current_node_id TEXT,
			current_node_type TEXT,
			detect_reason TEXT,
			recovery_action TEXT,
			status TEXT,
			details TEXT,
			error_message TEXT,
			started_at DATETIME,
			finished_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE approval_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_instance_id TEXT,
			node_id TEXT,
			status TEXT,
			updated_at DATETIME
		);
	`)

	tenantID := uuid.New()
	instanceID := uuid.New()
	flowID := uuid.New()
	approvalID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	nodes := `[{"id":"approval_node","type":"approval","name":"审批","config":{}},{"id":"end_node","type":"end","name":"结束","config":{}}]`
	edges := `[{"source":"approval_node","sourceHandle":"approved","target":"end_node"}]`
	mustExecHealing(t, db, `
		INSERT INTO healing_flows (id, tenant_id, name, nodes, edges, is_active, auto_close_source_incident)
		VALUES (?, ?, 'flow', ?, ?, 1, 0)
	`, flowID.String(), tenantID.String(), nodes, edges)
	mustExecHealing(t, db, `
		INSERT INTO flow_instances (id, tenant_id, flow_id, status, current_node_id, context, node_states, flow_name, flow_nodes, flow_edges)
		VALUES (?, ?, ?, ?, ?, '{}', '{}', ?, ?, ?)
	`, instanceID.String(), tenantID.String(), flowID.String(), model.FlowInstanceStatusWaitingApproval, "approval_node", "flow", nodes, edges)
	mustExecHealing(t, db, `
		INSERT INTO approval_tasks (id, tenant_id, flow_instance_id, node_id, status)
		VALUES (?, ?, ?, ?, ?)
	`, approvalID.String(), tenantID.String(), instanceID.String(), "approval_node", model.ApprovalTaskStatusApproved)

	executor := &FlowExecutor{
		instanceRepo:   automationrepo.NewFlowInstanceRepositoryWithDB(db),
		approvalRepo:   automationrepo.NewApprovalTaskRepositoryWithDB(db),
		flowRepo:       automationrepo.NewHealingFlowRepositoryWithDB(db),
		flowLogRepo:    automationrepo.NewFlowLogRepositoryWithDB(db),
		recoveryRepo:   automationrepo.NewFlowRecoveryAttemptRepositoryWithDB(db),
		incidentCloser: &stubIncidentCloser{},
		eventBus:       GetEventBus(),
	}

	attempt, err := executor.RecoverInstance(ctx, instanceID, model.FlowRecoveryTriggerManual)
	if err != nil {
		t.Fatalf("RecoverInstance(): %v", err)
	}
	if attempt.Status != model.FlowRecoveryStatusSuccess {
		t.Fatalf("attempt status = %s, want success", attempt.Status)
	}

	var row struct {
		Status string
	}
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&row).Error; err != nil {
		t.Fatalf("read instance: %v", err)
	}
	if row.Status != model.FlowInstanceStatusCompleted {
		t.Fatalf("instance status = %s, want completed", row.Status)
	}
}
