package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	executionSvc "github.com/company/auto-healing/internal/modules/automation/service/execution"
	healingSvc "github.com/company/auto-healing/internal/modules/automation/service/healing"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"github.com/google/uuid"
)

func TestExecutionRunCancelAPIUpdatesRunStatus(t *testing.T) {
	db := newExecutionHealingHandlerTestDB(t)
	createExecutionHandlerSchema(t, db)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playbookID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	taskID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	runID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	insertExecutionHandlerRunFixture(t, db, tenantID, playbookID, taskID, runID, "running")

	handler := &ExecutionHandler{service: executionSvc.NewServiceWithDB(db)}
	t.Cleanup(handler.Shutdown)

	router := newTenantAuthorizedRouter(tenantID, "task:cancel")
	registerTenantExecutionRunRoutes(router.Group("/api/v1/tenant/execution-runs"), handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenant/execution-runs/"+runID.String()+"/cancel", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Body.String() == "" {
		t.Fatal("response body should not be empty")
	}

	assertExecutionRunState(t, db, runID, "cancelled")
	assertExecutionLogCount(t, db, runID, 1)
}

func TestHealingInstanceCancelAPIUpdatesInstanceApprovalAndIncident(t *testing.T) {
	db := newExecutionHealingHandlerTestDB(t)
	createHealingHandlerSchema(t, db)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	tenantID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	instanceID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	incidentID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	approvalID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	insertHealingHandlerCancelFixture(t, db, tenantID, instanceID, incidentID, approvalID)

	handler := &HealingHandler{
		instanceRepo: automationrepo.NewFlowInstanceRepositoryWithDB(db),
		approvalRepo: automationrepo.NewApprovalTaskRepositoryWithDB(db),
		incidentRepo: incidentrepo.NewIncidentRepositoryWithDB(db),
		executor:     healingSvc.NewFlowExecutor(),
	}
	t.Cleanup(handler.Shutdown)

	router := newTenantAuthorizedRouter(tenantID, "healing:flows:update")
	registerTenantHealingInstanceRoutes(router.Group("/api/v1/tenant/healing/instances"), handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenant/healing/instances/"+instanceID.String()+"/cancel", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Body.String() == "" {
		t.Fatal("response body should not be empty")
	}

	assertHealingInstanceStatus(t, db, instanceID, model.FlowInstanceStatusCancelled)
	assertApprovalTaskStatus(t, db, approvalID, model.ApprovalTaskStatusCancelled)
	assertIncidentHealingStatus(t, db, incidentID, "dismissed")
}
