package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/google/uuid"
)

func TestRegisteredDashboardOverviewRouteRejectsUnauthorizedSections(t *testing.T) {
	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: uuid.NewString(),
		permissions:     []string{"dashboard:view"},
	})
	New(Dependencies{Dashboard: &DashboardHandler{}}).RegisterTenantRoutes(router.Group("/api/v1/tenant"))

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
	New(Dependencies{Workbench: &WorkbenchHandler{}}).RegisterCommonRoutes(router.Group("/api/v1/common"))

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
		prefRepo: engagementrepo.NewUserPreferenceRepositoryWithDB(db),
	}
	userID := uuid.NewString()

	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          userID,
		defaultTenantID: uuid.NewString(),
	})
	New(Dependencies{Preference: handler}).RegisterCommonRoutes(router.Group("/common"))

	putJSON(t, router, http.MethodPut, "/common/user/preferences", `{"preferences":{"theme":"alpha"}}`, "")
	putJSON(t, router, http.MethodPatch, "/common/user/preferences", `{"preferences":{"layout":"grid"}}`, "")
	resp := getJSON(t, router, "/common/user/preferences", "")

	data := resp.Data.(map[string]interface{})
	preferences := data["preferences"].(map[string]interface{})
	if preferences["theme"] != "alpha" || preferences["layout"] != "grid" {
		t.Fatalf("preferences = %#v, want merged values", preferences)
	}
}
