package httpapi

import (
	"path/filepath"
	"testing"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/modules/automation/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newExecutionHealingHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "execution-healing-handler.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func newTenantAuthorizedRouter(tenantID uuid.UUID, permissions ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.PermissionsKey, permissions)
		ctx := platformrepo.WithTenantID(c.Request.Context(), tenantID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	return router
}

func createExecutionHandlerSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecHandlerSQL(t, db, `
		CREATE TABLE playbooks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			repository_id TEXT,
			name TEXT,
			status TEXT,
			file_path TEXT,
			variables TEXT
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			playbook_id TEXT NOT NULL,
			target_hosts TEXT,
			executor_type TEXT,
			extra_vars TEXT,
			secrets_source_ids TEXT,
			playbook_variables_snapshot TEXT,
			needs_review BOOLEAN,
			changed_variables TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE execution_runs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			task_id TEXT NOT NULL,
			status TEXT NOT NULL,
			exit_code INTEGER,
			stdout TEXT,
			stderr TEXT,
			stats TEXT,
			triggered_by TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			runtime_target_hosts TEXT,
			runtime_secrets_source_ids TEXT,
			runtime_extra_vars TEXT,
			runtime_skip_notification BOOLEAN
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE execution_logs (
			id TEXT PRIMARY KEY,
			tenant_id TEXT,
			run_id TEXT NOT NULL,
			workflow_instance_id TEXT,
			node_execution_id TEXT,
			log_level TEXT NOT NULL,
			stage TEXT NOT NULL,
			message TEXT NOT NULL,
			host TEXT,
			task_name TEXT,
			play_name TEXT,
			details TEXT,
			sequence INTEGER NOT NULL,
			created_at DATETIME
		);
	`)
}

func createHealingHandlerSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecHandlerSQL(t, db, `
		CREATE TABLE healing_flows (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			nodes TEXT,
			edges TEXT,
			close_policy TEXT
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE healing_rules (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			plugin_id TEXT,
			source_plugin_name TEXT,
			external_id TEXT,
			title TEXT,
			description TEXT,
			severity TEXT,
			priority TEXT,
			status TEXT,
			category TEXT,
			affected_ci TEXT,
			affected_service TEXT,
			assignee TEXT,
			reporter TEXT,
			raw_data TEXT,
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
	mustExecHandlerSQL(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT,
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
	mustExecHandlerSQL(t, db, `
		CREATE TABLE approval_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_instance_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			status TEXT NOT NULL,
			decision_comment TEXT,
			decided_at DATETIME,
			updated_at DATETIME
		);
	`)
}

func insertExecutionHandlerRunFixture(t *testing.T, db *gorm.DB, tenantID, playbookID, taskID, runID uuid.UUID, status string) {
	t.Helper()
	mustExecHandlerSQL(t, db, `INSERT INTO playbooks (id, tenant_id, name, status, file_path, variables) VALUES (?, ?, ?, ?, ?, ?)`,
		playbookID.String(), tenantID.String(), "playbook", "ready", "site.yml", "[]")
	mustExecHandlerSQL(t, db, `INSERT INTO execution_tasks (id, tenant_id, name, playbook_id, target_hosts, executor_type, extra_vars, secrets_source_ids, playbook_variables_snapshot, needs_review, changed_variables) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID.String(), tenantID.String(), "task", playbookID.String(), "127.0.0.1", "local", "{}", "[]", "[]", false, "[]")
	mustExecHandlerSQL(t, db, `INSERT INTO execution_runs (id, tenant_id, task_id, status, triggered_by, runtime_target_hosts, runtime_secrets_source_ids, runtime_extra_vars, runtime_skip_notification) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID.String(), tenantID.String(), taskID.String(), status, "manual", "127.0.0.1", "[]", "{}", false)
}

func insertHealingHandlerCancelFixture(t *testing.T, db *gorm.DB, tenantID, instanceID, incidentID, approvalID uuid.UUID) {
	t.Helper()
	mustExecHandlerSQL(t, db, `INSERT INTO incidents (id, tenant_id, healing_status, scanned) VALUES (?, ?, ?, ?)`,
		incidentID.String(), tenantID.String(), "processing", true)
	mustExecHandlerSQL(t, db, `INSERT INTO flow_instances (id, tenant_id, incident_id, status, context, node_states, flow_name, flow_nodes, flow_edges) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		instanceID.String(), tenantID.String(), incidentID.String(), model.FlowInstanceStatusWaitingApproval, "{}", "{}", "flow", "[]", "[]")
	mustExecHandlerSQL(t, db, `INSERT INTO approval_tasks (id, tenant_id, flow_instance_id, node_id, status) VALUES (?, ?, ?, ?, ?)`,
		approvalID.String(), tenantID.String(), instanceID.String(), "approval-node", model.ApprovalTaskStatusPending)
}

func mustExecHandlerSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}

func assertExecutionRunState(t *testing.T, db *gorm.DB, runID uuid.UUID, want string) {
	t.Helper()
	type row struct{ Status string }
	var run row
	if err := db.Table("execution_runs").Select("status").Where("id = ?", runID.String()).Scan(&run).Error; err != nil {
		t.Fatalf("read execution run: %v", err)
	}
	if run.Status != want {
		t.Fatalf("execution run status = %s, want %s", run.Status, want)
	}
}

func assertExecutionLogCount(t *testing.T, db *gorm.DB, runID uuid.UUID, want int64) {
	t.Helper()
	var count int64
	if err := db.Table("execution_logs").Where("run_id = ?", runID.String()).Count(&count).Error; err != nil {
		t.Fatalf("count execution logs: %v", err)
	}
	if count != want {
		t.Fatalf("execution log count = %d, want %d", count, want)
	}
}

func assertHealingInstanceStatus(t *testing.T, db *gorm.DB, instanceID uuid.UUID, want string) {
	t.Helper()
	var status string
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read flow instance: %v", err)
	}
	if status != want {
		t.Fatalf("flow instance status = %s, want %s", status, want)
	}
}

func assertApprovalTaskStatus(t *testing.T, db *gorm.DB, approvalID uuid.UUID, want string) {
	t.Helper()
	var status string
	if err := db.Table("approval_tasks").Select("status").Where("id = ?", approvalID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read approval task: %v", err)
	}
	if status != want {
		t.Fatalf("approval task status = %s, want %s", status, want)
	}
}

func assertIncidentHealingStatus(t *testing.T, db *gorm.DB, incidentID uuid.UUID, want string) {
	t.Helper()
	var status string
	if err := db.Table("incidents").Select("healing_status").Where("id = ?", incidentID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read incident: %v", err)
	}
	if status != want {
		t.Fatalf("incident healing_status = %s, want %s", status, want)
	}
}
