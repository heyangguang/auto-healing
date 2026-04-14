package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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
		problem_template TEXT,
		solution_template TEXT,
		verification_template TEXT,
		conclusion_template TEXT,
		steps_render_mode TEXT,
		steps_max_count INTEGER,
		step_output_max_length INTEGER,
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

func TestCloseIncidentIntegrationRendersStructuredSolutionTemplate(t *testing.T) {
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
	insertStructuredIncidentSolutionTemplate(t, db, tenantID, templateID)
	mustExecIncidentServiceSQL(t, db, `UPDATE incident_solution_templates SET steps_render_mode = 'detailed' WHERE id = ?`, templateID.String())

	svc := NewIncidentServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	_, err := svc.CloseIncident(ctx, CloseIncidentParams{
		IncidentID:         incidentID,
		SolutionTemplateID: &templateID,
		TemplateVars: model.JSON{
			"flow": map[string]any{"name": "服务恢复流程"},
			"execution": map[string]any{
				"run_id":  "run-structured",
				"status":  "completed",
				"message": "执行成功",
			},
			"steps": []map[string]any{
				{"title": "提取工单主机", "summary": "提取工单主机，识别 1 台主机：app-1", "status": "completed"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CloseIncident() error = %v", err)
	}

	req := waitIncidentServiceCloseRequest(t, closeRequest)
	if req["resolution"] != "AHS 已完成自动修复：service integration" {
		t.Fatalf("resolution = %#v", req["resolution"])
	}
	workNotes, _ := req["work_notes"].(string)
	if !strings.Contains(workNotes, "问题说明：") {
		t.Fatalf("work_notes missing problem section: %#v", workNotes)
	}
	if !strings.Contains(workNotes, "执行步骤：\n1. 提取工单主机") {
		t.Fatalf("work_notes missing rendered steps: %#v", workNotes)
	}
}

func TestCloseIncidentIntegrationKeepsFullStructuredStepDetail(t *testing.T) {
	db := newIncidentServiceIntegrationDB(t)
	bindIncidentServiceIntegrationDB(t, db)
	createIncidentServiceIntegrationSchema(t, db)

	closeRequest := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		payload := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
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
	insertStructuredIncidentSolutionTemplate(t, db, tenantID, templateID)
	mustExecIncidentServiceSQL(t, db, `UPDATE incident_solution_templates SET steps_render_mode = 'detailed' WHERE id = ?`, templateID.String())

	svc := NewIncidentServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	_, err := svc.CloseIncident(ctx, CloseIncidentParams{
		IncidentID:         incidentID,
		SolutionTemplateID: &templateID,
		TemplateVars: model.JSON{
			"flow": map[string]any{"name": "服务恢复流程"},
			"execution": map[string]any{
				"status":  "completed",
				"message": "执行成功",
				"run_id":  "run-full-detail",
			},
			"steps": []map[string]any{
				{
					"title":   "执行服务恢复",
					"summary": "执行成功",
					"status":  "completed",
					"detail": strings.Join([]string{
						"- task 1：成功",
						"- task 2：成功",
						"- task 3：成功",
						"- task 4：成功",
						"- task 5：成功",
						"- task 6：成功",
						"- task 7：成功",
						"- task 8：成功",
						"- task 9：成功",
						"- task 10：成功",
						"- task 11：成功",
						"- task 12：成功",
						"- task 13：成功",
					}, "\n"),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CloseIncident() error = %v", err)
	}

	req := waitIncidentServiceCloseRequest(t, closeRequest)
	workNotes, _ := req["work_notes"].(string)
	if strings.Contains(workNotes, "其余输出已省略") {
		t.Fatalf("work_notes should keep full detail: %#v", workNotes)
	}
	if !strings.Contains(workNotes, "   - task 13：成功") {
		t.Fatalf("work_notes missing indented final detail: %#v", workNotes)
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
			id, tenant_id, name, description, problem_template, solution_template, verification_template, conclusion_template,
			steps_render_mode, steps_max_count, step_output_max_length, resolution_template, work_notes_template,
			default_close_code, default_close_status, created_at, updated_at
		) VALUES (?, ?, 'tmpl', 'demo', '', '', '', '', 'summary', 6, 240, ?, ?, 'auto_healed', 'resolved', ?, ?)
	`,
		templateID.String(),
		tenantID.String(),
		`AHS 已完成处理：{{ incident.title }}`,
		`流程={{ flow.name }}；run={{ execution.run_id }}；结果={{ execution.message }}`,
		now,
		now,
	)
}

func insertStructuredIncidentSolutionTemplate(t *testing.T, db *gorm.DB, tenantID, templateID uuid.UUID) {
	t.Helper()

	now := time.Now().UTC()
	mustExecIncidentServiceSQL(t, db, `
		INSERT INTO incident_solution_templates (
			id, tenant_id, name, description, problem_template, solution_template, verification_template, conclusion_template,
			steps_render_mode, steps_max_count, step_output_max_length, resolution_template, work_notes_template,
			default_close_code, default_close_status, created_at, updated_at
		) VALUES (?, ?, 'structured-tmpl', 'demo', ?, ?, ?, ?, 'summary', 6, 240, '', '', 'auto_healed', 'resolved', ?, ?)
	`,
		templateID.String(),
		tenantID.String(),
		`故障工单：{{ incident.title }}`,
		`AHS 按标准方案执行流程 {{ flow.name }}`,
		`执行状态：{{ execution.status }}`,
		`AHS 已完成自动修复：{{ incident.title }}`,
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
