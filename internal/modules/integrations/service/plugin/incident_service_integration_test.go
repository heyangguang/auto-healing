package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var incidentServiceSchemaStatements = []string{
	`CREATE TABLE plugins (
		id TEXT PRIMARY KEY NOT NULL,
		tenant_id TEXT,
		name TEXT,
		type TEXT,
		description TEXT,
		version TEXT,
		config TEXT,
		field_mapping TEXT,
		sync_filter TEXT,
		sync_enabled BOOLEAN,
		sync_interval_minutes INTEGER,
		last_sync_at DATETIME,
		next_sync_at DATETIME,
		max_failures INTEGER,
		consecutive_failures INTEGER,
		pause_reason TEXT,
		status TEXT,
		error_message TEXT,
		created_at DATETIME,
		updated_at DATETIME
	);`,
	`CREATE TABLE incidents (
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
	);`,
	`CREATE TABLE incident_writeback_logs (
		id TEXT PRIMARY KEY NOT NULL,
		tenant_id TEXT,
		incident_id TEXT NOT NULL,
		plugin_id TEXT,
		external_id TEXT,
		action TEXT,
		trigger_source TEXT,
		status TEXT,
		request_method TEXT,
		request_url TEXT,
		request_payload TEXT,
		response_status_code INTEGER,
		response_body TEXT,
		error_message TEXT,
		operator_user_id TEXT,
		operator_name TEXT,
		flow_instance_id TEXT,
		execution_run_id TEXT,
		started_at DATETIME,
		finished_at DATETIME,
		created_at DATETIME
	);`,
	`CREATE TABLE incident_solution_templates (
		id TEXT PRIMARY KEY NOT NULL,
		tenant_id TEXT,
		name TEXT,
		description TEXT,
		resolution_template TEXT,
		work_notes_template TEXT,
		default_close_code TEXT,
		default_close_status TEXT,
		created_at DATETIME,
		updated_at DATETIME
	);`,
}

func TestCloseIncidentIntegrationUpdatesSourceAndLocalState(t *testing.T) {
	db := newIncidentServiceIntegrationDB(t)
	createIncidentServiceIntegrationSchema(t, db)
	bindIncidentServiceIntegrationDB(t, db)

	closeRequest := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode close request: %v", err)
		}
		payload["method"] = r.Method
		payload["path"] = r.URL.Path
		closeRequest <- payload
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tenantID := uuid.New()
	pluginID := uuid.New()
	incidentID := uuid.New()
	insertIncidentServicePlugin(t, db, tenantID, pluginID, server.URL)
	insertIncidentServiceIncident(t, db, incidentID, tenantID, pluginID)

	svc := NewIncidentServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	resp, err := svc.CloseIncident(ctx, CloseIncidentParams{
		IncidentID:    incidentID,
		Resolution:    "done",
		WorkNotes:     "integration",
		CloseCode:     "auto",
		CloseStatus:   "closed",
		TriggerSource: platformmodel.IncidentWritebackTriggerManualClose,
		OperatorName:  "tester",
	})
	if err != nil {
		t.Fatalf("CloseIncident() error = %v", err)
	}
	if !resp.SourceUpdated {
		t.Fatal("SourceUpdated = false, want true")
	}

	req := waitIncidentServiceCloseRequest(t, closeRequest)
	if req["path"] != "/integration-close/INC-9000" {
		t.Fatalf("path = %v, want /integration-close/INC-9000", req["path"])
	}

	incident, err := incidentrepo.NewIncidentRepositoryWithDB(db).GetByID(ctx, incidentID)
	if err != nil {
		t.Fatalf("reload incident: %v", err)
	}
	if incident.Status != "closed" {
		t.Fatalf("status = %q, want closed", incident.Status)
	}
	if incident.HealingStatus != "healed" {
		t.Fatalf("healing_status = %q, want healed", incident.HealingStatus)
	}
	var logRow struct {
		Status        string
		RequestMethod string
		RequestURL    string
	}
	if err := db.Raw(`SELECT status, request_method, request_url FROM incident_writeback_logs WHERE incident_id = ?`, incidentID.String()).Scan(&logRow).Error; err != nil {
		t.Fatalf("query writeback log: %v", err)
	}
	if logRow.Status != platformmodel.IncidentWritebackStatusSuccess {
		t.Fatalf("writeback status = %q, want success", logRow.Status)
	}
	if logRow.RequestMethod != "POST" {
		t.Fatalf("request_method = %q, want POST", logRow.RequestMethod)
	}
	if logRow.RequestURL != server.URL+"/integration-close/INC-9000" {
		t.Fatalf("request_url = %q, want %q", logRow.RequestURL, server.URL+"/integration-close/INC-9000")
	}
}

func TestCloseIncidentIntegrationKeepsLocalStateWhenPluginLookupFails(t *testing.T) {
	db := newIncidentServiceIntegrationDB(t)
	createIncidentServiceIntegrationSchema(t, db)
	bindIncidentServiceIntegrationDB(t, db)

	tenantID := uuid.New()
	pluginID := uuid.New()
	incidentID := uuid.New()
	insertIncidentServiceIncident(t, db, incidentID, tenantID, pluginID)
	mustExecIncidentServiceSQL(t, db, `DROP TABLE plugins;`)

	svc := NewIncidentServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	if _, err := svc.CloseIncident(ctx, CloseIncidentParams{
		IncidentID:    incidentID,
		Resolution:    "done",
		WorkNotes:     "integration",
		CloseCode:     "auto",
		CloseStatus:   "closed",
		TriggerSource: platformmodel.IncidentWritebackTriggerManualClose,
		OperatorName:  "tester",
	}); err == nil {
		t.Fatal("CloseIncident() expected plugin lookup error")
	}

	state := loadIncidentServiceState(t, db, incidentID)
	if state.Status != "open" {
		t.Fatalf("status = %q, want open", state.Status)
	}
	if state.HealingStatus != "pending" {
		t.Fatalf("healing_status = %q, want pending", state.HealingStatus)
	}
}

func TestCloseIncidentIntegrationRendersSolutionTemplate(t *testing.T) {
	db := newIncidentServiceIntegrationDB(t)
	createIncidentServiceIntegrationSchema(t, db)
	bindIncidentServiceIntegrationDB(t, db)

	closeRequest := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode close request: %v", err)
		}
		closeRequest <- payload
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tenantID := uuid.New()
	pluginID := uuid.New()
	incidentID := uuid.New()
	templateID := uuid.New()
	insertIncidentServicePlugin(t, db, tenantID, pluginID, server.URL)
	insertIncidentServiceIncident(t, db, incidentID, tenantID, pluginID)
	insertIncidentSolutionTemplate(t, db, tenantID, templateID)

	svc := NewIncidentServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	_, err := svc.CloseIncident(ctx, CloseIncidentParams{
		IncidentID:         incidentID,
		SolutionTemplateID: &templateID,
		TemplateVars: model.JSON{
			"flow": map[string]any{"name": "服务恢复流程"},
			"execution": map[string]any{
				"run_id":  "run-1",
				"status":  "success",
				"message": "执行完成",
			},
		},
	})
	if err != nil {
		t.Fatalf("CloseIncident() error = %v", err)
	}

	req := waitIncidentServiceCloseRequest(t, closeRequest)
	if req["resolution"] != "AHS 已完成处理：service integration" {
		t.Fatalf("resolution = %#v", req["resolution"])
	}
	if req["work_notes"] != "流程=服务恢复流程；run=run-1；结果=执行完成" {
		t.Fatalf("work_notes = %#v", req["work_notes"])
	}
	if req["close_code"] != "auto_healed" {
		t.Fatalf("close_code = %#v", req["close_code"])
	}
	if req["close_status"] != "resolved" {
		t.Fatalf("close_status = %#v", req["close_status"])
	}
}

func newIncidentServiceIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "incident-service.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func bindIncidentServiceIntegrationDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	logger.Init(&config.LogConfig{
		Console: config.ConsoleLogConfig{Enabled: true, Format: "text"},
	})
	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })
}

func createIncidentServiceIntegrationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()

	for _, statement := range incidentServiceSchemaStatements {
		mustExecIncidentServiceSQL(t, db, statement)
	}
}

func insertIncidentServicePlugin(t *testing.T, db *gorm.DB, tenantID, pluginID uuid.UUID, baseURL string) {
	t.Helper()

	now := time.Now().UTC()
	configJSON := `{"close_incident_url":"` + baseURL + `/integration-close/{external_id}","close_incident_method":"POST"}`
	mustExecIncidentServiceSQL(t, db, `
		INSERT INTO plugins (
			id, tenant_id, name, type, version, config, field_mapping, sync_enabled,
			sync_interval_minutes, max_failures, consecutive_failures, status, created_at, updated_at
		) VALUES (?, ?, 'itsm-plugin', 'itsm', '1.0.0', ?, '{}', 1, 5, 5, 0, 'active', ?, ?)
	`, pluginID.String(), tenantID.String(), configJSON, now, now)
}

func insertIncidentServiceIncident(t *testing.T, db *gorm.DB, incidentID, tenantID, pluginID uuid.UUID) {
	t.Helper()

	now := time.Now().UTC()
	rawData, err := model.JSON{"id": "INC-9000"}.Value()
	if err != nil {
		t.Fatalf("marshal raw_data: %v", err)
	}
	mustExecIncidentServiceSQL(t, db, `
		INSERT INTO incidents (
			id, tenant_id, plugin_id, source_plugin_name, external_id, title, status, raw_data,
			healing_status, scanned, created_at, updated_at
		) VALUES (?, ?, ?, 'itsm-plugin', 'INC-9000', 'service integration', 'open', ?, 'pending', 0, ?, ?)
	`, incidentID.String(), tenantID.String(), pluginID.String(), rawData, now, now)
}

func insertIncidentSolutionTemplate(t *testing.T, db *gorm.DB, tenantID, templateID uuid.UUID) {
	t.Helper()

	now := time.Now().UTC()
	mustExecIncidentServiceSQL(t, db, `
		INSERT INTO incident_solution_templates (
			id, tenant_id, name, description, resolution_template, work_notes_template,
			default_close_code, default_close_status, created_at, updated_at
		) VALUES (?, ?, 'tmpl', 'demo', ?, ?, 'auto_healed', 'resolved', ?, ?)
	`,
		templateID.String(),
		tenantID.String(),
		`AHS 已完成处理：{{ incident.title }}`,
		`流程={{ flow.name }}；run={{ execution.run_id }}；结果={{ execution.message }}`,
		now,
		now,
	)
}

func waitIncidentServiceCloseRequest(t *testing.T, closeRequest <-chan map[string]any) map[string]any {
	t.Helper()

	select {
	case req := <-closeRequest:
		return req
	case <-time.After(time.Second):
		t.Fatal("expected close incident request")
		return nil
	}
}

func loadIncidentServiceState(t *testing.T, db *gorm.DB, incidentID uuid.UUID) struct {
	Status        string
	HealingStatus string
} {
	t.Helper()

	var state struct {
		Status        string
		HealingStatus string
	}
	if err := db.Raw(`SELECT status, healing_status FROM incidents WHERE id = ?`, incidentID.String()).Scan(&state).Error; err != nil {
		t.Fatalf("query incident state: %v", err)
	}
	return state
}

func mustExecIncidentServiceSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()

	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql: %v\nsql=%s", err, sql)
	}
}
