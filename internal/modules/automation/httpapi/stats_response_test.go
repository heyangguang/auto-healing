package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	executionSvc "github.com/company/auto-healing/internal/modules/automation/service/execution"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type statsResponseEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestExecutionStatsEndpointsReturnTopLevelData(t *testing.T) {
	db := newExecutionStatsTestDB(t)
	createExecutionHandlerSchema(t, db)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	tenantID := uuid.New()
	playbookID := uuid.New()
	taskA := uuid.New()
	taskB := uuid.New()
	insertExecutionStatsFixtures(t, db, tenantID, playbookID, taskA, taskB)

	handler := &ExecutionHandler{service: executionSvc.NewService()}
	t.Cleanup(handler.Shutdown)

	router := newExecutionStatsRouter(tenantID, handler)

	assertDataArrayResponse(t, router, "/api/v1/tenant/execution-runs/trigger-distribution")
}

func newExecutionStatsRouter(tenantID uuid.UUID, handler *ExecutionHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.PermissionsKey, []string{"task:list"})
		ctx := platformrepo.WithTenantID(c.Request.Context(), tenantID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	group := router.Group("/api/v1/tenant/execution-runs")
	group.GET("/trigger-distribution", handler.GetTriggerDistribution)
	return router
}

func assertDataArrayResponse(t *testing.T, router http.Handler, path string) {
	t.Helper()
	resp := issueExecutionStatsRequest(t, router, path)
	var items []map[string]any
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		t.Fatalf("decode array data for %s: %v; raw=%s", path, err, string(resp.Data))
	}
	if len(items) == 0 {
		t.Fatalf("%s returned empty data array", path)
	}
	if _, exists := items[0]["items"]; exists {
		t.Fatalf("%s returned nested items wrapper: %+v", path, items[0])
	}
}

func issueExecutionStatsRequest(t *testing.T, router http.Handler, path string) statsResponseEnvelope {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d; body=%s", path, recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var resp statsResponseEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response for %s: %v", path, err)
	}
	if resp.Code != 0 || resp.Message != "success" {
		t.Fatalf("%s response = %+v, want code=0 message=success", path, resp)
	}
	return resp
}

func newExecutionStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "execution-stats.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func insertExecutionStatsFixtures(t *testing.T, db *gorm.DB, tenantID, playbookID, taskA, taskB uuid.UUID) {
	t.Helper()
	now := time.Now().UTC()
	mustExecHandlerSQL(t, db, `INSERT INTO playbooks (id, tenant_id, name, status, file_path, variables) VALUES (?, ?, ?, ?, ?, ?)`,
		playbookID.String(), tenantID.String(), "playbook", "ready", "site.yml", "[]")
	mustExecHandlerSQL(t, db, `INSERT INTO execution_tasks (id, tenant_id, name, playbook_id, target_hosts, executor_type, extra_vars, secrets_source_ids, playbook_variables_snapshot, needs_review, changed_variables, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskA.String(), tenantID.String(), "task-a", playbookID.String(), "10.0.0.1", "local", "{}", "[]", "[]", false, "[]", now, now,
		taskB.String(), tenantID.String(), "task-b", playbookID.String(), "10.0.0.2", "docker", "{}", "[]", "[]", true, "[]", now, now)
	mustExecHandlerSQL(t, db, `INSERT INTO execution_runs (id, tenant_id, task_id, status, triggered_by, started_at, completed_at, created_at, runtime_target_hosts, runtime_secrets_source_ids, runtime_extra_vars, runtime_skip_notification) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), tenantID.String(), taskA.String(), "success", "manual", now.Add(-2*time.Hour), now.Add(-time.Hour), now.Add(-2*time.Hour), "10.0.0.1", "[]", "{}", false,
		uuid.NewString(), tenantID.String(), taskA.String(), "failed", "schedule", now.Add(-30*time.Minute), now.Add(-20*time.Minute), now.Add(-30*time.Minute), "10.0.0.1", "[]", "{}", false)
}
