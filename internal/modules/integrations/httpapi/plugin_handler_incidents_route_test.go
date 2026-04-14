package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/google/uuid"
)

const handlerIncidentPluginSchema = `
CREATE TABLE plugins (
	id TEXT PRIMARY KEY NOT NULL,
	tenant_id TEXT,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	description TEXT,
	version TEXT NOT NULL,
	config TEXT NOT NULL,
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
);`

const handlerIncidentSchema = `
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
);`

const handlerIncidentWritebackLogSchema = `
CREATE TABLE incident_writeback_logs (
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
);`

const handlerIncidentSolutionTemplateSchema = `
CREATE TABLE incident_solution_templates (
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
);`

func TestGetIncidentAPIUses404ForMissingIncident(t *testing.T) {
	router, _, _ := newIncidentRouteTestHarness(t, []string{"plugin:list"}, handlerIncidentPluginSchema, handlerIncidentSchema)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenant/incidents/"+uuid.NewString(), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertIncidentRouteResponse(t, recorder, http.StatusNotFound, "工单不存在")
}

func TestResetIncidentScanAPIUses404ForMissingIncident(t *testing.T) {
	router, _, _ := newIncidentRouteTestHarness(t, []string{"plugin:sync"}, handlerIncidentPluginSchema, handlerIncidentSchema)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenant/incidents/"+uuid.NewString()+"/reset-scan", nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertIncidentRouteResponse(t, recorder, http.StatusNotFound, "工单不存在")
}

func TestCloseIncidentAPIUses500ForRepositoryFailure(t *testing.T) {
	router, _, _ := newIncidentRouteTestHarness(t, []string{"plugin:sync"}, handlerIncidentPluginSchema, handlerIncidentWritebackLogSchema)
	req := newIncidentJSONRequest(t, http.MethodPost, "/api/v1/tenant/incidents/"+uuid.NewString()+"/close", CloseIncidentRequest{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertIncidentRouteResponse(t, recorder, http.StatusInternalServerError, "关闭工单失败")
}

func TestCloseIncidentEndToEndUpdatesSourceAndLocalState(t *testing.T) {
	router, db, tenantID := newIncidentRouteTestHarness(t, []string{"plugin:sync"}, handlerIncidentPluginSchema, handlerIncidentSchema, handlerIncidentWritebackLogSchema)
	pluginID := uuid.New()
	incidentID := uuid.New()
	now := time.Now().UTC()
	var gotPath string
	var gotPayload map[string]any

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode close payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(source.Close)

	insertHandlerIncidentPlugin(t, db, tenantID, pluginID, model.JSON{
		"auth_type":          "none",
		"close_incident_url": source.URL + "/incidents/{external_id}/close",
	})
	insertHandlerIncident(t, db, tenantID, pluginID, incidentID, "INC-200", "open", "pending", &now)

	req := newIncidentJSONRequest(t, http.MethodPost, "/api/v1/tenant/incidents/"+incidentID.String()+"/close", CloseIncidentRequest{
		Resolution: "fixed",
		WorkNotes:  "done",
		CloseCode:  "resolved",
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	payload := decodeIncidentSuccessResponse(t, recorder)
	data, ok := payload.Data.(map[string]any)
	if !ok {
		t.Fatalf("response data type = %T, want map[string]any", payload.Data)
	}
	if data["source_updated"] != true {
		t.Fatalf("source_updated = %#v, want true", data["source_updated"])
	}
	if data["local_status"] != "healed" {
		t.Fatalf("local_status = %#v, want healed", data["local_status"])
	}
	if gotPath != "/incidents/INC-200/close" {
		t.Fatalf("close-back path = %q, want %q", gotPath, "/incidents/INC-200/close")
	}
	if gotPayload["close_status"] != "resolved" {
		t.Fatalf("close payload close_status = %#v, want resolved", gotPayload["close_status"])
	}

	status, healingStatus := readHandlerIncidentState(t, db, incidentID)
	if status != "resolved" {
		t.Fatalf("incident status = %q, want resolved", status)
	}
	if healingStatus != "healed" {
		t.Fatalf("incident healing_status = %q, want healed", healingStatus)
	}
}

func TestCloseIncidentWithSolutionTemplateRendersRequestPayload(t *testing.T) {
	router, db, tenantID := newIncidentRouteTestHarness(t, []string{"plugin:sync"}, handlerIncidentPluginSchema, handlerIncidentSchema, handlerIncidentWritebackLogSchema, handlerIncidentSolutionTemplateSchema)
	pluginID := uuid.New()
	incidentID := uuid.New()
	templateID := uuid.New()
	now := time.Now().UTC()
	var gotPayload map[string]any

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode close payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(source.Close)

	insertHandlerIncidentPlugin(t, db, tenantID, pluginID, model.JSON{
		"auth_type":          "none",
		"close_incident_url": source.URL + "/incidents/{external_id}/close",
	})
	insertHandlerIncident(t, db, tenantID, pluginID, incidentID, "INC-201", "open", "pending", &now)
	if err := db.Exec(`
		INSERT INTO incident_solution_templates (
			id, tenant_id, name, description, resolution_template, work_notes_template,
			default_close_code, default_close_status, created_at, updated_at
		) VALUES (?, ?, 'tmpl', 'demo', ?, ?, 'auto_healed', 'resolved', ?, ?)
	`, templateID.String(), tenantID.String(), `处理完成：{{ incident.title }}`, `run={{ execution.run_id }}`, now, now).Error; err != nil {
		t.Fatalf("insert template: %v", err)
	}

	req := newIncidentJSONRequest(t, http.MethodPost, "/api/v1/tenant/incidents/"+incidentID.String()+"/close", CloseIncidentRequest{
		SolutionTemplateID: &templateID,
		TemplateVars: model.JSON{
			"execution": map[string]any{"run_id": "run-201"},
		},
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if gotPayload["resolution"] != "处理完成：incident-title" {
		t.Fatalf("resolution = %#v", gotPayload["resolution"])
	}
	if gotPayload["work_notes"] != "run=run-201" {
		t.Fatalf("work_notes = %#v", gotPayload["work_notes"])
	}
}

func TestListIncidentWritebackLogsReturnsIncidentScopedLogs(t *testing.T) {
	router, db, tenantID := newIncidentRouteTestHarness(t, []string{"plugin:list"}, handlerIncidentSchema, handlerIncidentWritebackLogSchema)
	incidentID := uuid.New()
	otherIncidentID := uuid.New()
	now := time.Now().UTC()

	if err := db.Exec(`
		INSERT INTO incidents (id, tenant_id, external_id, title, raw_data, created_at, updated_at)
		VALUES (?, ?, 'INC-1', 'one', '{}', ?, ?), (?, ?, 'INC-2', 'two', '{}', ?, ?)
	`, incidentID.String(), tenantID.String(), now, now, otherIncidentID.String(), tenantID.String(), now, now).Error; err != nil {
		t.Fatalf("insert incidents: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO incident_writeback_logs (id, tenant_id, incident_id, external_id, action, trigger_source, status, created_at)
		VALUES (?, ?, ?, 'INC-1', 'close', 'manual_close', 'success', ?), (?, ?, ?, 'INC-2', 'close', 'manual_close', 'failed', ?)
	`, uuid.NewString(), tenantID.String(), incidentID.String(), now, uuid.NewString(), tenantID.String(), otherIncidentID.String(), now).Error; err != nil {
		t.Fatalf("insert writeback logs: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenant/incidents/"+incidentID.String()+"/writeback-logs", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	payload := decodeIncidentSuccessResponse(t, recorder)
	items, ok := payload.Data.([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", payload.Data)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	logItem, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("item type = %T, want map[string]any", items[0])
	}
	if logItem["external_id"] != "INC-1" {
		t.Fatalf("external_id = %#v, want INC-1", logItem["external_id"])
	}
}
