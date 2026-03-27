package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	respPkg "github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestDashboardOverviewRouteRejectsUnauthorizedSections(t *testing.T) {
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:      uuid.NewString(),
		permissions: []string{"plugin:list"},
	})
	router.GET("/tenant/dashboard/overview", (&DashboardHandler{}).GetOverview)

	req := httptest.NewRequest(http.MethodGet, "/tenant/dashboard/overview?sections=plugins,notifications", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	resp := decodeOwnedScopeResponse(t, recorder)
	if resp.Code != respPkg.CodeForbidden {
		t.Fatalf("code = %d, want %d", resp.Code, respPkg.CodeForbidden)
	}
	if !strings.Contains(resp.Message, "notifications") {
		t.Fatalf("message = %q, want unauthorized section details", resp.Message)
	}
}

func TestWorkbenchOverviewRouteRequiresTenantContext(t *testing.T) {
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:      uuid.NewString(),
		permissions: []string{"plugin:list"},
	})
	router.GET("/common/workbench/overview", (&WorkbenchHandler{}).GetOverview)

	req := httptest.NewRequest(http.MethodGet, "/common/workbench/overview", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	resp := decodeOwnedScopeResponse(t, recorder)
	if !strings.Contains(resp.Message, "租户上下文") {
		t.Fatalf("message = %q, want tenant context error", resp.Message)
	}
}

func TestWorkbenchOverviewRouteHidesPlaybooksWithoutPlaybookPermission(t *testing.T) {
	db := newPreferenceTestDB(t)
	createWorkbenchOverviewScopeSchema(t, db)
	tenantID := uuid.New()
	mustExecPreferenceTest(t, db, `INSERT INTO playbooks (id, tenant_id, status) VALUES (?, ?, ?)`, uuid.NewString(), tenantID.String(), "draft")

	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: tenantID.String(),
		permissions:     []string{"plugin:list"},
	})
	router.GET("/common/workbench/overview", (&WorkbenchHandler{
		repo: repository.NewWorkbenchRepository(db),
	}).GetOverview)

	req := httptest.NewRequest(http.MethodGet, "/common/workbench/overview", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	resp := decodeOwnedScopeResponse(t, recorder)
	data := resp.Data.(map[string]interface{})
	resourceOverview := data["resource_overview"].(map[string]interface{})
	playbooks := resourceOverview["playbooks"].(map[string]interface{})
	if playbooks["total"] != float64(0) {
		t.Fatalf("playbooks.total = %#v, want 0", playbooks["total"])
	}
	if _, exists := playbooks["needs_review"]; exists {
		t.Fatalf("playbooks = %#v, want needs_review omitted", playbooks)
	}
}

func TestWorkbenchScheduleCalendarRouteRejectsInvalidMonth(t *testing.T) {
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: uuid.NewString(),
		permissions:     []string{"task:list"},
	})
	router.GET("/common/workbench/schedule-calendar", (&WorkbenchHandler{}).GetScheduleCalendar)

	req := httptest.NewRequest(http.MethodGet, "/common/workbench/schedule-calendar?year=2026&month=13", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	resp := decodeOwnedScopeResponse(t, recorder)
	if !strings.Contains(resp.Message, "month") {
		t.Fatalf("message = %q, want invalid month error", resp.Message)
	}
}

func TestGlobalSearchRouteRequiresTenantContext(t *testing.T) {
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:      uuid.NewString(),
		permissions: []string{"plugin:list"},
	})
	router.GET("/common/search", (&SearchHandler{}).GlobalSearch)

	req := httptest.NewRequest(http.MethodGet, "/common/search?q=demo", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	resp := decodeOwnedScopeResponse(t, recorder)
	if !strings.Contains(resp.Message, "租户上下文") {
		t.Fatalf("message = %q, want tenant context error", resp.Message)
	}
}

func TestGlobalSearchRouteRejectsEmptyKeyword(t *testing.T) {
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: uuid.NewString(),
		permissions:     []string{"plugin:list"},
	})
	router.GET("/common/search", (&SearchHandler{}).GlobalSearch)

	req := httptest.NewRequest(http.MethodGet, "/common/search", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	resp := decodeOwnedScopeResponse(t, recorder)
	if !strings.Contains(resp.Message, "搜索关键词不能为空") {
		t.Fatalf("message = %q, want empty keyword error", resp.Message)
	}
}

func createWorkbenchOverviewScopeSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecPreferenceTest(t, db, `
		CREATE TABLE playbooks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT
		);
	`)
	mustExecPreferenceTest(t, db, `
		CREATE TABLE cmdb_items (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT,
			type TEXT
		);
	`)
	mustExecPreferenceTest(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			scanned BOOLEAN,
			healing_status TEXT,
			created_at DATETIME
		);
	`)
	mustExecPreferenceTest(t, db, `
		CREATE TABLE secrets_sources (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			type TEXT
		);
	`)
}
