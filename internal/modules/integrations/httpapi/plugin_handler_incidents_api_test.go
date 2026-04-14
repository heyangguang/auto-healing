package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	pluginservice "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	incidentListPermission = "plugin:list"
	incidentSyncPermission = "plugin:sync"
)

var pluginIncidentSchemaStatements = []string{
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
}

type pluginIncidentRow struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	PluginID   *uuid.UUID
	ExternalID string
	Title      string
	Status     string
	Healing    string
	Scanned    bool
}

func TestIncidentAPIGetReturnsNotFound(t *testing.T) {
	db := newPluginIncidentHandlerTestDB(t)
	createPluginIncidentSchema(t, db)
	bindPluginIncidentHandlerTestDB(t, db)

	handler := newPluginIncidentHandlerTestHandler(t, db)
	router := newPluginIncidentHandlerRouter(uuid.New(), []string{incidentListPermission})
	router.GET("/incidents/:id", middleware.RequirePermission(incidentListPermission), handler.GetIncident)

	recorder := issuePluginIncidentRequest(router, http.MethodGet, "/incidents/"+uuid.NewString(), "")
	assertPluginIncidentHTTPResponse(t, recorder, http.StatusNotFound, response.CodeNotFound, "工单不存在")
}

func TestIncidentAPIGetReturnsInternalErrorOnRepositoryFailure(t *testing.T) {
	db := newPluginIncidentHandlerTestDB(t)
	bindPluginIncidentHandlerTestDB(t, db)

	handler := newPluginIncidentHandlerTestHandler(t, db)
	router := newPluginIncidentHandlerRouter(uuid.New(), []string{incidentListPermission})
	router.GET("/incidents/:id", middleware.RequirePermission(incidentListPermission), handler.GetIncident)

	recorder := issuePluginIncidentRequest(router, http.MethodGet, "/incidents/"+uuid.NewString(), "")
	assertPluginIncidentHTTPResponse(t, recorder, http.StatusInternalServerError, response.CodeInternal, "获取工单详情失败")
}

func TestIncidentAPICloseReturnsNotFound(t *testing.T) {
	db := newPluginIncidentHandlerTestDB(t)
	createPluginIncidentSchema(t, db)
	bindPluginIncidentHandlerTestDB(t, db)

	handler := newPluginIncidentHandlerTestHandler(t, db)
	router := newPluginIncidentHandlerRouter(uuid.New(), []string{incidentSyncPermission})
	router.POST("/incidents/:id/close", middleware.RequirePermission(incidentSyncPermission), handler.CloseIncident)

	recorder := issuePluginIncidentRequest(router, http.MethodPost, "/incidents/"+uuid.NewString()+"/close", `{"resolution":"done"}`)
	assertPluginIncidentHTTPResponse(t, recorder, http.StatusNotFound, response.CodeNotFound, "工单不存在")
}

func TestIncidentAPIResetScanReturnsNotFound(t *testing.T) {
	db := newPluginIncidentHandlerTestDB(t)
	createPluginIncidentSchema(t, db)
	bindPluginIncidentHandlerTestDB(t, db)

	handler := newPluginIncidentHandlerTestHandler(t, db)
	router := newPluginIncidentHandlerRouter(uuid.New(), []string{incidentSyncPermission})
	router.POST("/incidents/:id/reset-scan", middleware.RequirePermission(incidentSyncPermission), handler.ResetIncidentScan)

	recorder := issuePluginIncidentRequest(router, http.MethodPost, "/incidents/"+uuid.NewString()+"/reset-scan", "")
	assertPluginIncidentHTTPResponse(t, recorder, http.StatusNotFound, response.CodeNotFound, "工单不存在")
}

func newPluginIncidentHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "plugin-incident-handler.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func bindPluginIncidentHandlerTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	logger.Init(&config.LogConfig{})
	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })
}

func createPluginIncidentSchema(t *testing.T, db *gorm.DB) {
	t.Helper()

	for _, statement := range pluginIncidentSchemaStatements {
		mustExecPluginIncidentSQL(t, db, statement)
	}
}

func newPluginIncidentHandlerTestHandler(t *testing.T, db *gorm.DB) *PluginHandler {
	t.Helper()

	gin.SetMode(gin.TestMode)
	handler := NewPluginHandlerWithDeps(PluginHandlerDeps{
		PluginService:   pluginservice.NewServiceWithDB(db),
		IncidentService: pluginservice.NewIncidentServiceWithDB(db),
	})
	t.Cleanup(handler.Shutdown)
	return handler
}

func newPluginIncidentHandlerRouter(tenantID uuid.UUID, permissions []string) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.TenantIDKey, tenantID.String())
		c.Set(middleware.PermissionsKey, permissions)
		c.Request = c.Request.WithContext(platformrepo.WithTenantID(c.Request.Context(), tenantID))
		c.Next()
	})
	return router
}

func issuePluginIncidentRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func assertPluginIncidentHTTPResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantHTTP, wantCode int, wantMessage string) {
	t.Helper()

	if recorder.Code != wantHTTP {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, wantHTTP, recorder.Body.String())
	}

	var payload response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != wantCode {
		t.Fatalf("code = %d, want %d", payload.Code, wantCode)
	}
	if payload.Message != wantMessage {
		t.Fatalf("message = %q, want %q", payload.Message, wantMessage)
	}
}

func mustExecPluginIncidentSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()

	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql: %v\nsql=%s", err, sql)
	}
}

func insertPluginIncidentRecord(t *testing.T, db *gorm.DB, row pluginIncidentRow) {
	t.Helper()

	now := time.Now().UTC()
	mustExecPluginIncidentSQL(t, db, `
		INSERT INTO incidents (
			id, tenant_id, plugin_id, source_plugin_name, external_id, title, status, raw_data,
			healing_status, scanned, created_at, updated_at
		) VALUES (?, ?, ?, 'itsm-plugin', ?, ?, ?, '{}', ?, ?, ?, ?)
	`, row.ID.String(), row.TenantID.String(), nullableUUID(row.PluginID), row.ExternalID, row.Title, row.Status, row.Healing, row.Scanned, now, now)
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}
