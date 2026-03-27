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
	"github.com/company/auto-healing/internal/modules/integrations/model"
	pluginservice "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newIncidentRouteTestHarness(t *testing.T, permissions []string, schemas ...string) (*gin.Engine, *gorm.DB, uuid.UUID) {
	t.Helper()
	logger.Init(&config.LogConfig{Console: config.ConsoleLogConfig{Enabled: true, Format: "text"}})
	db := openIncidentRouteTestDB(t)
	createIncidentRouteSchema(t, db, schemas...)
	useIncidentRouteDB(t, db)

	tenantID := uuid.New()
	gin.SetMode(gin.TestMode)
	pluginHandler := NewPluginHandlerWithDeps(PluginHandlerDeps{
		PluginService:   pluginservice.NewServiceWithDB(db),
		IncidentService: pluginservice.NewIncidentServiceWithDB(db),
	})
	t.Cleanup(pluginHandler.Shutdown)
	router := gin.New()
	router.Use(injectIncidentRouteContext(tenantID, permissions))
	registerTenantIncidentRoutes(router.Group("/api/v1/tenant/incidents"), pluginHandler, &stubIncidentHealingActions{})
	return router, db, tenantID
}

type stubIncidentHealingActions struct{}

func (s *stubIncidentHealingActions) TriggerIncidentManually(c *gin.Context) {}
func (s *stubIncidentHealingActions) DismissIncident(c *gin.Context)         {}

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
		c.Request = c.Request.WithContext(platformrepo.WithTenantID(c.Request.Context(), tenantID))
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
