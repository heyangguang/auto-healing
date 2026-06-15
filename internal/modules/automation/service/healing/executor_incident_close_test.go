package healing

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestCompleteSkipsAutoCloseWhenClosePolicyIsDisabled(t *testing.T) {
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
			close_policy TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT NOT NULL,
			incident_id TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			node_states TEXT,
			updated_at DATETIME,
			completed_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			status TEXT,
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

	tenantID := uuid.New()
	flowID := uuid.New()
	instanceID := uuid.New()
	incidentID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	mustExecHealing(t, db, `
		INSERT INTO healing_flows (id, tenant_id, name, nodes, edges, is_active)
		VALUES (?, ?, 'service flow', '[]', '[]', 1)
	`, flowID.String(), tenantID.String())
	mustExecHealing(t, db, `
		INSERT INTO flow_instances (id, tenant_id, flow_id, incident_id, status, node_states)
		VALUES (?, ?, ?, ?, ?, '{}')
	`, instanceID.String(), tenantID.String(), flowID.String(), incidentID.String(), model.FlowInstanceStatusRunning)
	mustExecHealing(t, db, `
		INSERT INTO incidents (id, tenant_id, healing_status, status)
		VALUES (?, ?, 'processing', 'new')
	`, incidentID.String(), tenantID.String())

	closer := &stubIncidentCloser{}
	executor := &FlowExecutor{
		instanceRepo:   automationrepo.NewFlowInstanceRepositoryWithDB(db),
		flowRepo:       automationrepo.NewHealingFlowRepositoryWithDB(db),
		flowLogRepo:    automationrepo.NewFlowLogRepositoryWithDB(db),
		incidentCloser: closer,
		eventBus:       GetEventBus(),
	}
	instance := &model.FlowInstance{
		ID:         instanceID,
		FlowID:     flowID,
		IncidentID: &incidentID,
		Context: model.JSON{
			"execution_result": map[string]interface{}{
				"run": map[string]interface{}{
					"run_id": uuid.NewString(),
				},
			},
		},
	}

	if err := executor.complete(ctx, instance); err != nil {
		t.Fatalf("complete(): %v", err)
	}
	if closer.callCount != 0 {
		t.Fatalf("callCount = %d, want 0", closer.callCount)
	}
}

func TestCompleteAutoClosesSourceIncidentUsesClosePolicyTemplate(t *testing.T) {
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
			close_policy TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT NOT NULL,
			incident_id TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			node_states TEXT,
			updated_at DATETIME,
			completed_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			status TEXT,
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

	tenantID := uuid.New()
	flowID := uuid.New()
	instanceID := uuid.New()
	incidentID := uuid.New()
	templateID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	mustExecHealing(t, db, `
		INSERT INTO healing_flows (id, tenant_id, name, nodes, edges, is_active, close_policy)
		VALUES (?, ?, 'service flow', '[]', '[]', 1, ?)
	`, flowID.String(), tenantID.String(), `{"enabled":true,"solution_template_id":"`+templateID.String()+`","default_close_status":"closed","default_close_code":"auto_healed"}`)
	mustExecHealing(t, db, `
		INSERT INTO flow_instances (id, tenant_id, flow_id, incident_id, status, node_states)
		VALUES (?, ?, ?, ?, ?, '{}')
	`, instanceID.String(), tenantID.String(), flowID.String(), incidentID.String(), model.FlowInstanceStatusRunning)
	mustExecHealing(t, db, `
		INSERT INTO incidents (id, tenant_id, healing_status, status)
		VALUES (?, ?, 'processing', 'new')
	`, incidentID.String(), tenantID.String())

	closer := &stubIncidentCloser{}
	executor := &FlowExecutor{
		instanceRepo:   automationrepo.NewFlowInstanceRepositoryWithDB(db),
		flowRepo:       automationrepo.NewHealingFlowRepositoryWithDB(db),
		flowLogRepo:    automationrepo.NewFlowLogRepositoryWithDB(db),
		incidentCloser: closer,
		eventBus:       GetEventBus(),
	}
	instance := &model.FlowInstance{
		ID:         instanceID,
		FlowID:     flowID,
		IncidentID: &incidentID,
		Context: model.JSON{
			"execution_result": map[string]interface{}{
				"status":       "success",
				"message":      "任务执行完成",
				"task_id":      "task-1",
				"target_hosts": "10.0.0.1",
				"run": map[string]interface{}{
					"run_id": uuid.NewString(),
					"stats":  map[string]interface{}{"ok": 1},
				},
			},
		},
	}

	if err := executor.complete(ctx, instance); err != nil {
		t.Fatalf("complete(): %v", err)
	}
	if closer.lastParams.SolutionTemplateID == nil || *closer.lastParams.SolutionTemplateID != templateID {
		t.Fatalf("solution_template_id = %v, want %s", closer.lastParams.SolutionTemplateID, templateID)
	}
	if closer.lastParams.CloseStatus != "closed" {
		t.Fatalf("close_status = %q, want closed", closer.lastParams.CloseStatus)
	}
	flowVars, ok := closer.lastParams.TemplateVars["flow"].(map[string]any)
	if !ok || flowVars["name"] != "service flow" {
		t.Fatalf("flow template vars = %#v", closer.lastParams.TemplateVars["flow"])
	}
}

func TestCompleteSkipsAutoCloseWhenExecutionResultFailed(t *testing.T) {
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
			close_policy TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT NOT NULL,
			incident_id TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			node_states TEXT,
			updated_at DATETIME,
			completed_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			status TEXT,
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

	tenantID := uuid.New()
	flowID := uuid.New()
	instanceID := uuid.New()
	incidentID := uuid.New()
	templateID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	mustExecHealing(t, db, `
		INSERT INTO healing_flows (id, tenant_id, name, nodes, edges, is_active, close_policy)
		VALUES (?, ?, 'service flow', '[]', '[]', 1, ?)
	`, flowID.String(), tenantID.String(), `{"enabled":true,"solution_template_id":"`+templateID.String()+`","default_close_status":"resolved","default_close_code":"auto_healed"}`)
	mustExecHealing(t, db, `
		INSERT INTO flow_instances (id, tenant_id, flow_id, incident_id, status, node_states)
		VALUES (?, ?, ?, ?, ?, '{}')
	`, instanceID.String(), tenantID.String(), flowID.String(), incidentID.String(), model.FlowInstanceStatusRunning)
	mustExecHealing(t, db, `
		INSERT INTO incidents (id, tenant_id, healing_status, status)
		VALUES (?, ?, 'processing', 'new')
	`, incidentID.String(), tenantID.String())

	closer := &stubIncidentCloser{}
	executor := &FlowExecutor{
		instanceRepo:   automationrepo.NewFlowInstanceRepositoryWithDB(db),
		flowRepo:       automationrepo.NewHealingFlowRepositoryWithDB(db),
		flowLogRepo:    automationrepo.NewFlowLogRepositoryWithDB(db),
		incidentCloser: closer,
		eventBus:       GetEventBus(),
	}
	instance := &model.FlowInstance{
		ID:         instanceID,
		FlowID:     flowID,
		IncidentID: &incidentID,
		Context: model.JSON{
			"execution_result": map[string]interface{}{
				"status":  "failed",
				"message": "任务执行失败",
				"run": map[string]interface{}{
					"run_id": uuid.NewString(),
				},
			},
		},
	}

	if err := executor.complete(ctx, instance); err != nil {
		t.Fatalf("complete(): %v", err)
	}
	if closer.callCount != 0 {
		t.Fatalf("callCount = %d, want 0", closer.callCount)
	}
}

type stubIncidentCloser struct {
	callCount  int
	lastParams IncidentCloseParams
}

func (s *stubIncidentCloser) CloseIncident(_ context.Context, params IncidentCloseParams) (*IncidentCloseResult, error) {
	s.callCount++
	s.lastParams = params
	return &IncidentCloseResult{
		LocalStatus:   "healed",
		SourceUpdated: true,
	}, nil
}
