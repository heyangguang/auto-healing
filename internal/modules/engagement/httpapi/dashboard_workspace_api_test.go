package httpapi

import (
	"net/http"
	"testing"

	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/google/uuid"
)

func TestDashboardConfigRouteScopesVisibleWorkspacesByPermission(t *testing.T) {
	db := newPreferenceTestDB(t)
	scenario := seedDashboardWorkspaceVisibilityScenario(t, db, true)
	handler := &DashboardHandler{
		repo:   engagementrepo.NewDashboardRepositoryWithDB(db),
		wsRepo: engagementrepo.NewWorkspaceRepositoryWithDB(db),
	}

	viewerWorkspaces := fetchDashboardConfigWorkspaces(t, handler, scenario.userID, scenario.tenantID, []string{"dashboard:view"})
	assertDashboardWorkspaceSet(t, viewerWorkspaces, map[string]bool{
		scenario.defaultID.String(): true,
		scenario.visibleID.String(): false,
	})

	managerWorkspaces := fetchDashboardConfigWorkspaces(
		t,
		handler,
		scenario.userID,
		scenario.tenantID,
		[]string{"dashboard:view", "dashboard:workspace:manage"},
	)
	assertDashboardWorkspaceSet(t, managerWorkspaces, map[string]bool{
		scenario.defaultID.String(): true,
		scenario.visibleID.String(): false,
		scenario.hiddenID.String():  true,
	})
}

func TestDashboardWorkspaceListRouteReturnsVisibleSubsetForViewer(t *testing.T) {
	db := newPreferenceTestDB(t)
	scenario := seedDashboardWorkspaceVisibilityScenario(t, db, false)
	handler := &DashboardHandler{
		wsRepo: engagementrepo.NewWorkspaceRepositoryWithDB(db),
	}

	workspaces := fetchDashboardWorkspaceList(t, handler, scenario.userID, scenario.tenantID, []string{"dashboard:view"})
	assertDashboardWorkspaceSet(t, workspaces, map[string]bool{
		scenario.defaultID.String(): true,
		scenario.visibleID.String(): false,
	})
}

func TestDashboardWorkspaceUpdateRejectsProtectedFields(t *testing.T) {
	db := newPreferenceTestDB(t)
	createDashboardWorkspaceAPISchema(t, db)

	tenantID := uuid.New()
	workspaceID := uuid.New()
	insertDashboardWorkspaceAPIWorkspace(t, db, tenantID, workspaceID, "default", false, false)

	handler := &DashboardHandler{
		wsRepo: engagementrepo.NewWorkspaceRepositoryWithDB(db),
	}
	router := newDashboardWorkspaceManageRouter(tenantID)
	router.PUT("/tenant/dashboard/workspaces/:id", handler.UpdateSystemWorkspace)

	resp := issueDashboardWorkspaceJSON(
		t,
		router,
		http.MethodPut,
		"/tenant/dashboard/workspaces/"+workspaceID.String(),
		`{"name":"renamed","is_default":true}`,
	)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
	}
}

func TestDashboardWorkspaceDeleteRejectsDefaultWorkspace(t *testing.T) {
	db := newPreferenceTestDB(t)
	createDashboardWorkspaceAPISchema(t, db)

	tenantID := uuid.New()
	workspaceID := uuid.New()
	insertDashboardWorkspaceAPIWorkspace(t, db, tenantID, workspaceID, "default", true, false)

	handler := &DashboardHandler{
		wsRepo: engagementrepo.NewWorkspaceRepositoryWithDB(db),
	}
	router := newDashboardWorkspaceManageRouter(tenantID)
	router.DELETE("/tenant/dashboard/workspaces/:id", handler.DeleteSystemWorkspace)

	resp := issueDashboardWorkspaceJSON(t, router, http.MethodDelete, "/tenant/dashboard/workspaces/"+workspaceID.String(), "")
	if resp.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusConflict, resp.Body.String())
	}
}
