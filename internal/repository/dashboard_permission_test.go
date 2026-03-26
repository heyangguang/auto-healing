package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestGetHealingSectionRespectsSubPermissions(t *testing.T) {
	db := newSQLiteTestDB(t)
	createHealingDashboardSchema(t, db)

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	now := time.Now().UTC().Format(time.RFC3339)
	mustExec(t, db, `INSERT INTO healing_flows (id, tenant_id, name, is_active, created_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), tenantID.String(), "flow-a", true, now)
	mustExec(t, db, `INSERT INTO healing_rules (id, tenant_id, name, is_active, trigger_mode, created_at) VALUES (?, ?, ?, ?, ?, ?)`, uuid.NewString(), tenantID.String(), "rule-a", true, "auto", now)
	mustExec(t, db, `INSERT INTO flow_instances (id, tenant_id, flow_id, flow_name, status, created_at) VALUES (?, ?, ?, ?, ?, ?)`, uuid.NewString(), tenantID.String(), uuid.NewString(), "instance-a", "running", now)
	mustExec(t, db, `INSERT INTO approval_tasks (id, tenant_id, flow_instance_id, node_id, status, created_at) VALUES (?, ?, ?, ?, ?, ?)`, uuid.NewString(), tenantID.String(), uuid.NewString(), "node-a", "pending", now)
	mustExec(t, db, `INSERT INTO incidents (id, tenant_id, scanned, matched_rule_id, healing_flow_instance_id, title, severity, affected_ci, created_at) VALUES (?, ?, ?, ?, NULL, ?, ?, ?, ?)`, uuid.NewString(), tenantID.String(), true, uuid.NewString(), "incident-a", "high", "ci-a", now)

	repo := &DashboardRepository{db: db}
	section, err := repo.GetHealingSection(WithTenantID(context.Background(), tenantID), []string{"healing:instances:view"})
	if err != nil {
		t.Fatalf("GetHealingSection() error = %v", err)
	}
	if section.InstancesTotal == 0 || section.InstancesRunning == 0 {
		t.Fatalf("instances stats not populated: %#v", section)
	}
	if section.FlowsTotal != 0 || section.RulesTotal != 0 || section.PendingApprovals != 0 || section.PendingTriggers != 0 {
		t.Fatalf("restricted healing fields leaked: %#v", section)
	}
	if len(section.PendingApprovalList) != 0 || len(section.PendingTriggerList) != 0 || len(section.FlowTop10) != 0 {
		t.Fatalf("restricted healing lists leaked: %#v", section)
	}
}

func TestGetHealingSectionFlowTop10RespectsTenantScope(t *testing.T) {
	db := newSQLiteTestDB(t)
	createHealingDashboardSchema(t, db)

	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	flowB := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO healing_flows (id, tenant_id, name, is_active, created_at) VALUES (?, ?, ?, ?, ?)`, flowB.String(), tenantB.String(), "flow-b", true, now)
	mustExec(t, db, `INSERT INTO flow_instances (id, tenant_id, flow_id, flow_name, status, created_at) VALUES (?, ?, ?, ?, ?, ?)`, uuid.NewString(), tenantA.String(), flowB.String(), "instance-a", "completed", now)

	repo := &DashboardRepository{db: db}
	section, err := repo.GetHealingSection(WithTenantID(context.Background(), tenantA), []string{"healing:flows:view"})
	if err != nil {
		t.Fatalf("GetHealingSection() error = %v", err)
	}
	if len(section.FlowTop10) != 0 {
		t.Fatalf("GetHealingSection() leaked cross-tenant flow names: %#v", section.FlowTop10)
	}
}

func createHealingDashboardSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE healing_flows (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			is_active BOOLEAN,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE healing_rules (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			is_active BOOLEAN,
			trigger_mode TEXT,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT,
			flow_name TEXT,
			status TEXT,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE approval_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_instance_id TEXT,
			node_id TEXT,
			status TEXT,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			scanned BOOLEAN,
			matched_rule_id TEXT,
			healing_flow_instance_id TEXT,
			title TEXT,
			severity TEXT,
			affected_ci TEXT,
			created_at DATETIME
		);
	`)
}
