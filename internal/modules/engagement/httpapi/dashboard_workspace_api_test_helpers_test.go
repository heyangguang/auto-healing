package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type dashboardWorkspacePayload struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsDefault  bool   `json:"is_default"`
	IsReadonly bool   `json:"is_readonly"`
}

type dashboardWorkspaceVisibilityScenario struct {
	tenantID  uuid.UUID
	userID    uuid.UUID
	defaultID uuid.UUID
	visibleID uuid.UUID
	hiddenID  uuid.UUID
}

func seedDashboardWorkspaceVisibilityScenario(t *testing.T, db *gorm.DB, hiddenReadonly bool) dashboardWorkspaceVisibilityScenario {
	t.Helper()
	createDashboardWorkspaceAPISchema(t, db)

	scenario := dashboardWorkspaceVisibilityScenario{
		tenantID:  uuid.New(),
		userID:    uuid.New(),
		defaultID: uuid.New(),
		visibleID: uuid.New(),
		hiddenID:  uuid.New(),
	}
	roleID := uuid.New()

	insertDashboardWorkspaceAPIWorkspace(t, db, scenario.tenantID, scenario.defaultID, "default", true, true)
	insertDashboardWorkspaceAPIWorkspace(t, db, scenario.tenantID, scenario.visibleID, "visible", false, false)
	insertDashboardWorkspaceAPIWorkspace(t, db, scenario.tenantID, scenario.hiddenID, "hidden", false, hiddenReadonly)
	insertDashboardWorkspaceAPIRoleAssignment(t, db, scenario.tenantID, roleID, scenario.visibleID)
	insertDashboardWorkspaceAPIUserRole(t, db, scenario.tenantID, scenario.userID, roleID)
	return scenario
}

func fetchDashboardConfigWorkspaces(
	t *testing.T,
	handler *DashboardHandler,
	userID, tenantID uuid.UUID,
	permissions []string,
) []dashboardWorkspacePayload {
	t.Helper()
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          userID.String(),
		defaultTenantID: tenantID.String(),
		permissions:     permissions,
	})
	router.GET("/tenant/dashboard/config", handler.GetConfig)

	resp := issueDashboardWorkspaceJSON(t, router, http.MethodGet, "/tenant/dashboard/config", "")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	return decodeDashboardConfigWorkspaces(t, resp)
}

func fetchDashboardWorkspaceList(
	t *testing.T,
	handler *DashboardHandler,
	userID, tenantID uuid.UUID,
	permissions []string,
) []dashboardWorkspacePayload {
	t.Helper()
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          userID.String(),
		defaultTenantID: tenantID.String(),
		permissions:     permissions,
	})
	router.GET("/tenant/dashboard/workspaces", handler.ListSystemWorkspaces)

	resp := issueDashboardWorkspaceJSON(t, router, http.MethodGet, "/tenant/dashboard/workspaces", "")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	return decodeDashboardWorkspaceList(t, resp)
}

func newDashboardWorkspaceManageRouter(tenantID uuid.UUID) *gin.Engine {
	return newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: tenantID.String(),
		permissions:     []string{"dashboard:workspace:manage"},
	})
}

func createDashboardWorkspaceAPISchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range dashboardWorkspaceAPISchemaStatements() {
		mustExecDashboardWorkspaceAPI(t, db, stmt)
	}
}

func dashboardWorkspaceAPISchemaStatements() []string {
	return []string{
		`CREATE TABLE dashboard_configs (id TEXT PRIMARY KEY NOT NULL, user_id TEXT NOT NULL, tenant_id TEXT, config TEXT NOT NULL DEFAULT '{}', created_at DATETIME, updated_at DATETIME);`,
		`CREATE UNIQUE INDEX idx_dashboard_tenant_user ON dashboard_configs(user_id, tenant_id);`,
		`CREATE TABLE system_workspaces (id TEXT PRIMARY KEY NOT NULL, tenant_id TEXT, name TEXT NOT NULL, description TEXT, config TEXT NOT NULL DEFAULT '{}', is_default BOOLEAN NOT NULL DEFAULT FALSE, is_readonly BOOLEAN NOT NULL DEFAULT FALSE, created_by TEXT, created_at DATETIME, updated_at DATETIME);`,
		`CREATE TABLE role_workspaces (id TEXT PRIMARY KEY NOT NULL, tenant_id TEXT, role_id TEXT NOT NULL, workspace_id TEXT NOT NULL, created_at DATETIME);`,
		`CREATE TABLE user_platform_roles (id TEXT PRIMARY KEY NOT NULL, user_id TEXT NOT NULL, role_id TEXT NOT NULL, created_at DATETIME);`,
		`CREATE TABLE user_tenant_roles (id TEXT PRIMARY KEY NOT NULL, user_id TEXT NOT NULL, tenant_id TEXT NOT NULL, role_id TEXT NOT NULL, created_at DATETIME);`,
	}
}

func insertDashboardWorkspaceAPIWorkspace(
	t *testing.T,
	db *gorm.DB,
	tenantID, workspaceID uuid.UUID,
	name string,
	isDefault, isReadonly bool,
) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecDashboardWorkspaceAPI(
		t,
		db,
		`INSERT INTO system_workspaces (id, tenant_id, name, description, config, is_default, is_readonly, created_at, updated_at) VALUES (?, ?, ?, '', '{}', ?, ?, ?, ?)`,
		workspaceID.String(),
		tenantID.String(),
		name,
		isDefault,
		isReadonly,
		now,
		now,
	)
}

func insertDashboardWorkspaceAPIRoleAssignment(t *testing.T, db *gorm.DB, tenantID, roleID, workspaceID uuid.UUID) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecDashboardWorkspaceAPI(
		t,
		db,
		`INSERT INTO role_workspaces (id, tenant_id, role_id, workspace_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(),
		tenantID.String(),
		roleID.String(),
		workspaceID.String(),
		now,
	)
}

func insertDashboardWorkspaceAPIUserRole(t *testing.T, db *gorm.DB, tenantID, userID, roleID uuid.UUID) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecDashboardWorkspaceAPI(
		t,
		db,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(),
		userID.String(),
		tenantID.String(),
		roleID.String(),
		now,
	)
}

func mustExecDashboardWorkspaceAPI(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func issueDashboardWorkspaceJSON(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeDashboardConfigWorkspaces(t *testing.T, recorder *httptest.ResponseRecorder) []dashboardWorkspacePayload {
	t.Helper()
	var resp struct {
		Data struct {
			SystemWorkspaces []dashboardWorkspacePayload `json:"system_workspaces"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return resp.Data.SystemWorkspaces
}

func decodeDashboardWorkspaceList(t *testing.T, recorder *httptest.ResponseRecorder) []dashboardWorkspacePayload {
	t.Helper()
	var resp struct {
		Data []dashboardWorkspacePayload `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return resp.Data
}

func assertDashboardWorkspaceSet(t *testing.T, workspaces []dashboardWorkspacePayload, expectedReadonly map[string]bool) {
	t.Helper()
	if len(workspaces) != len(expectedReadonly) {
		t.Fatalf("workspace count = %d, want %d", len(workspaces), len(expectedReadonly))
	}
	for _, workspace := range workspaces {
		wantReadonly, ok := expectedReadonly[workspace.ID]
		if !ok {
			t.Fatalf("unexpected workspace id=%s", workspace.ID)
		}
		if workspace.IsReadonly != wantReadonly {
			t.Fatalf("workspace %s readonly=%v, want %v", workspace.ID, workspace.IsReadonly, wantReadonly)
		}
	}
}
