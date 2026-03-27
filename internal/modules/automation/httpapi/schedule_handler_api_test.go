package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	scheduleSvc "github.com/company/auto-healing/internal/modules/automation/service/schedule"
	respPkg "github.com/company/auto-healing/internal/pkg/response"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestScheduleHandlerGetTimelineRejectsInvalidDate(t *testing.T) {
	handler := &ScheduleHandler{service: &scheduleSvc.Service{}}
	router := gin.New()
	router.GET("/timeline", handler.GetTimeline)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/timeline?date=bad", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestScheduleHandlerStatsAndTimeline(t *testing.T) {
	db := openScheduleHandlerTestDB(t)
	createScheduleHandlerSchema(t, db)

	tenantID := uuid.New()
	taskID := uuid.New()
	scheduleID := uuid.New()
	now := time.Now().In(time.FixedZone("CST", 8*3600))
	nextRunAt := now.Add(2 * time.Hour)
	lastRunAt := now.Add(-2 * time.Hour)

	mustExecScheduleHandlerSQL(t, db, `
		INSERT INTO execution_tasks (id, tenant_id, name) VALUES (?, ?, ?)
	`, taskID.String(), tenantID.String(), "nightly task")
	mustExecScheduleHandlerSQL(t, db, `
		INSERT INTO execution_schedules (
			id, tenant_id, name, task_id, schedule_type, schedule_expr, scheduled_at, status, enabled,
			next_run_at, last_run_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, scheduleID.String(), tenantID.String(), "nightly", taskID.String(), "cron", "0 2 * * *", nil, "running", true, nextRunAt, lastRunAt, now, now)

	handler := &ScheduleHandler{service: scheduleSvc.NewServiceWithDeps(scheduleSvc.ServiceDeps{
		Repo:     automationrepo.NewScheduleRepositoryWithDB(db),
		ExecRepo: &automationrepo.ExecutionRepository{},
	})}
	router := newScheduleHandlerTestRouter(tenantID)
	router.GET("/stats", handler.GetStats)
	router.GET("/timeline", handler.GetTimeline)

	statsResp := issueScheduleHandlerRequest(t, router, "/stats")
	if statsResp.Code != respPkg.CodeSuccess {
		t.Fatalf("stats code = %d, want %d", statsResp.Code, respPkg.CodeSuccess)
	}
	stats := decodeScheduleStatsData(t, statsResp.Data)
	if got := int64(stats["total"].(float64)); got != 1 {
		t.Fatalf("stats.total = %d, want 1", got)
	}

	timelineResp := issueScheduleHandlerRequest(t, router, "/timeline?date="+now.Format("2006-01-02"))
	if timelineResp.Code != respPkg.CodeSuccess {
		t.Fatalf("timeline code = %d, want %d", timelineResp.Code, respPkg.CodeSuccess)
	}
	items := decodeScheduleTimelineItems(t, timelineResp.Data)
	if len(items) != 1 {
		t.Fatalf("timeline len = %d, want 1", len(items))
	}
	if items[0].ID != scheduleID || items[0].TaskName != "nightly task" {
		t.Fatalf("unexpected timeline item: %+v", items[0])
	}
}

func newScheduleHandlerTestRouter(tenantID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platformrepo.WithTenantID(c.Request.Context(), tenantID))
		c.Next()
	})
	return router
}

func openScheduleHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "schedule-handler.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createScheduleHandlerSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecScheduleHandlerSQL(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL
		)
	`)
	mustExecScheduleHandlerSQL(t, db, `
		CREATE TABLE execution_schedules (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			task_id TEXT NOT NULL,
			schedule_type TEXT,
			schedule_expr TEXT,
			scheduled_at DATETIME,
			status TEXT,
			enabled BOOLEAN,
			next_run_at DATETIME,
			last_run_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
}

func mustExecScheduleHandlerSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}

func issueScheduleHandlerRequest(t *testing.T, router http.Handler, path string) respPkg.Response {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	router.ServeHTTP(recorder, req)

	var resp respPkg.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return resp
}

func decodeScheduleStatsData(t *testing.T, data any) map[string]any {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal stats data: %v", err)
	}
	var stats map[string]any
	if err := json.Unmarshal(payload, &stats); err != nil {
		t.Fatalf("unmarshal stats data: %v; payload=%s", err, string(payload))
	}
	return stats
}

func decodeScheduleTimelineItems(t *testing.T, data any) []automationrepo.ScheduleTimelineItem {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal timeline data: %v", err)
	}
	var items []automationrepo.ScheduleTimelineItem
	if err := json.Unmarshal(payload, &items); err != nil {
		t.Fatalf("unmarshal timeline data: %v; payload=%s", err, string(payload))
	}
	return items
}
