package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

func TestRegisteredDashboardOverviewRouteRejectsUnauthorizedSections(t *testing.T) {
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: uuid.NewString(),
		permissions:     []string{"dashboard:view"},
	})
	registerTenantDashboardRoutes(router.Group("/api/v1/tenant/dashboard"), &DashboardHandler{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenant/dashboard/overview?sections=playbooks", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestRegisteredWorkbenchScheduleCalendarRouteRequiresTaskListPermission(t *testing.T) {
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: uuid.NewString(),
		permissions:     []string{"dashboard:view"},
	})
	workbench := router.Group("/api/v1/common/workbench")
	workbench.GET("/schedule-calendar", middleware.RequirePermission("task:list"), (&WorkbenchHandler{}).GetScheduleCalendar)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/common/workbench/schedule-calendar?year=2026&month=3", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestPreferencePatchRouteMergesStoredPreferences(t *testing.T) {
	db := newPreferenceTestDB(t)
	createUserPreferenceSchema(t, db)
	handler := &PreferenceHandler{
		prefRepo: repository.NewUserPreferenceRepositoryWithDB(db),
	}
	userID := uuid.NewString()

	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          userID,
		defaultTenantID: uuid.NewString(),
	})
	router.PUT("/common/user/preferences", handler.UpdatePreferences)
	router.PATCH("/common/user/preferences", handler.PatchPreferences)
	router.GET("/common/user/preferences", handler.GetPreferences)

	putJSON(t, router, http.MethodPut, "/common/user/preferences", `{"preferences":{"theme":"alpha"}}`, "")
	putJSON(t, router, http.MethodPatch, "/common/user/preferences", `{"preferences":{"layout":"grid"}}`, "")
	resp := getJSON(t, router, "/common/user/preferences", "")

	data := resp.Data.(map[string]interface{})
	preferences := data["preferences"].(map[string]interface{})
	if preferences["theme"] != "alpha" || preferences["layout"] != "grid" {
		t.Fatalf("preferences = %#v, want merged values", preferences)
	}
}
