package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
	router, _, _ := newIncidentRouteTestHarness(t, []string{"plugin:sync"}, handlerIncidentPluginSchema)
	req := newIncidentJSONRequest(t, http.MethodPost, "/api/v1/tenant/incidents/"+uuid.NewString()+"/close", CloseIncidentRequest{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertIncidentRouteResponse(t, recorder, http.StatusInternalServerError, "关闭工单失败")
}

func TestCloseIncidentEndToEndUpdatesSourceAndLocalState(t *testing.T) {
	router, db, tenantID := newIncidentRouteTestHarness(t, []string{"plugin:sync"}, handlerIncidentPluginSchema, handlerIncidentSchema)
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

func newIncidentRouteTestHarness(t *testing.T, permissions []string, schemas ...string) (*gin.Engine, *gorm.DB, uuid.UUID) {
	t.Helper()

	logger.Init(&config.LogConfig{
		Console: config.ConsoleLogConfig{Enabled: true, Format: "text"},
	})
	db := openIncidentRouteTestDB(t)
	createIncidentRouteSchema(t, db, schemas...)
	useIncidentRouteDB(t, db)

	tenantID := uuid.New()
	gin.SetMode(gin.TestMode)

	pluginHandler := NewPluginHandler()
	t.Cleanup(pluginHandler.Shutdown)
	router := gin.New()
	router.Use(injectIncidentRouteContext(tenantID, permissions))
	registerTenantIncidentRoutes(router.Group("/api/v1/tenant/incidents"), pluginHandler, &stubIncidentHealingActions{})
	return router, db, tenantID
}

type stubIncidentHealingActions struct{}

func (s *stubIncidentHealingActions) TriggerIncidentManually(c *gin.Context) {}

func (s *stubIncidentHealingActions) DismissIncident(c *gin.Context) {}

func openIncidentRouteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "incident-route.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createIncidentRouteSchema(t *testing.T, db *gorm.DB, statements ...string) {
	t.Helper()

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("exec sql failed: %v", err)
		}
	}
}

func useIncidentRouteDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })
}

func injectIncidentRouteContext(tenantID uuid.UUID, permissions []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.PermissionsKey, permissions)
		c.Set(middleware.TenantIDKey, tenantID.String())
		c.Request = c.Request.WithContext(repository.WithTenantID(c.Request.Context(), tenantID))
		c.Next()
	}
}

func newIncidentJSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func insertHandlerIncidentPlugin(t *testing.T, db *gorm.DB, tenantID, pluginID uuid.UUID, config model.JSON) {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Exec(`
		INSERT INTO plugins (id, tenant_id, name, type, version, config, sync_enabled, sync_interval_minutes, max_failures, consecutive_failures, status, created_at, updated_at)
		VALUES (?, ?, 'itsm-plugin', 'itsm', '1.0.0', ?, true, 5, 5, 0, 'active', ?, ?)
	`, pluginID.String(), tenantID.String(), config, now, now).Error; err != nil {
		t.Fatalf("insert plugin: %v", err)
	}
}

func insertHandlerIncident(t *testing.T, db *gorm.DB, tenantID, pluginID, incidentID uuid.UUID, externalID, status, healingStatus string, sourceUpdatedAt *time.Time) {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Exec(`
		INSERT INTO incidents (
			id, tenant_id, plugin_id, source_plugin_name, external_id, title, description, status, raw_data,
			healing_status, scanned, source_updated_at, created_at, updated_at
		) VALUES (?, ?, ?, 'itsm-plugin', ?, 'incident-title', 'incident-desc', ?, ?, ?, false, ?, ?, ?)
	`, incidentID.String(), tenantID.String(), pluginID.String(), externalID, status, model.JSON{"id": externalID}, healingStatus, sourceUpdatedAt, now, now).Error; err != nil {
		t.Fatalf("insert incident: %v", err)
	}
}

func readHandlerIncidentState(t *testing.T, db *gorm.DB, incidentID uuid.UUID) (string, string) {
	t.Helper()

	var result struct {
		Status        string
		HealingStatus string
	}
	if err := db.Raw(`SELECT status, healing_status FROM incidents WHERE id = ?`, incidentID.String()).Scan(&result).Error; err != nil {
		t.Fatalf("query incident state: %v", err)
	}
	return result.Status, result.HealingStatus
}

func decodeIncidentSuccessResponse(t *testing.T, recorder *httptest.ResponseRecorder) response.Response {
	t.Helper()

	var payload response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

func assertIncidentRouteResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantMessage string) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	var payload response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Message != wantMessage {
		t.Fatalf("message = %q, want %q", payload.Message, wantMessage)
	}
}
