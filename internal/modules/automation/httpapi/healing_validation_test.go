package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestCreateFlowRejectsInvalidNodesJSON(t *testing.T) {
	db := newExecutionHealingHandlerTestDB(t)
	createHealingValidationSchema(t, db)
	tenantID := uuid.New()
	handler := newHealingValidationTestHandler(db)
	router := newTenantAuthorizedRouter(tenantID)
	router.POST("/flows", handler.CreateFlow)

	req := httptest.NewRequest(http.MethodPost, "/flows", strings.NewReader(`{"name":"bad","nodes":{"id":"x"},"edges":[]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestCreateFlowRejectsMissingExecutionTaskReference(t *testing.T) {
	db := newExecutionHealingHandlerTestDB(t)
	createHealingValidationSchema(t, db)
	tenantID := uuid.New()
	handler := newHealingValidationTestHandler(db)
	router := newTenantAuthorizedRouter(tenantID)
	router.POST("/flows", handler.CreateFlow)

	missingTaskID := uuid.NewString()
	body := `{
		"name":"bad execution ref",
		"nodes":[
			{"id":"start","type":"start"},
			{"id":"exec","type":"execution","config":{"task_template_id":"` + missingTaskID + `"}}
		],
		"edges":[{"source":"start","target":"exec"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/flows", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "任务模板") {
		t.Fatalf("body = %s, want task template validation message", recorder.Body.String())
	}
}

func TestCreateFlowRejectsEdgeReferencingMissingNode(t *testing.T) {
	db := newExecutionHealingHandlerTestDB(t)
	createHealingValidationSchema(t, db)
	tenantID := uuid.New()
	handler := newHealingValidationTestHandler(db)
	router := newTenantAuthorizedRouter(tenantID)
	router.POST("/flows", handler.CreateFlow)

	body := `{
		"name":"bad edge",
		"nodes":[{"id":"start","type":"start"}],
		"edges":[{"source":"start","target":"missing"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/flows", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestCreateRuleRejectsInvalidTriggerMode(t *testing.T) {
	db := newExecutionHealingHandlerTestDB(t)
	createHealingValidationSchema(t, db)
	tenantID := uuid.New()
	handler := newHealingValidationTestHandler(db)
	router := newTenantAuthorizedRouter(tenantID)
	router.POST("/rules", handler.CreateRule)

	req := httptest.NewRequest(http.MethodPost, "/rules", strings.NewReader(`{"name":"bad","trigger_mode":"sometimes","conditions":[]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestCreateRuleRejectsMissingFlowReference(t *testing.T) {
	db := newExecutionHealingHandlerTestDB(t)
	createHealingValidationSchema(t, db)
	tenantID := uuid.New()
	handler := newHealingValidationTestHandler(db)
	router := newTenantAuthorizedRouter(tenantID)
	router.POST("/rules", handler.CreateRule)

	body := `{"name":"bad flow ref","flow_id":"` + uuid.NewString() + `","conditions":[]}`
	req := httptest.NewRequest(http.MethodPost, "/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "flow_id") {
		t.Fatalf("body = %s, want flow_id validation message", recorder.Body.String())
	}
}

func newHealingValidationTestHandler(db *gorm.DB) *HealingHandler {
	return &HealingHandler{
		flowRepo:      automationrepo.NewHealingFlowRepositoryWithDB(db),
		ruleRepo:      automationrepo.NewHealingRuleRepositoryWithDB(db),
		notifRepo:     engagementrepo.NewNotificationRepository(db),
		executionRepo: automationrepo.NewExecutionRepositoryWithDB(db),
		solutionRepo:  integrationrepo.NewIncidentSolutionTemplateRepositoryWithDB(db),
	}
}

func createHealingValidationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecHandlerSQL(t, db, `
		CREATE TABLE healing_flows (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			description TEXT,
			nodes TEXT,
			edges TEXT,
			is_active BOOLEAN,
			close_policy TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE healing_rules (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			description TEXT,
			priority INTEGER,
			trigger_mode TEXT,
			conditions TEXT,
			match_mode TEXT,
			flow_id TEXT,
			is_active BOOLEAN,
			last_run_at DATETIME,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			playbook_id TEXT,
			target_hosts TEXT,
			executor_type TEXT,
			extra_vars TEXT,
			secrets_source_ids TEXT,
			notification_config TEXT,
			playbook_variables_snapshot TEXT,
			needs_review BOOLEAN,
			changed_variables TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
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
		CREATE TABLE git_repositories (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE notification_templates (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE notification_channels (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT
		);
	`)
	mustExecHandlerSQL(t, db, `
		CREATE TABLE incident_solution_templates (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT
		);
	`)
}
