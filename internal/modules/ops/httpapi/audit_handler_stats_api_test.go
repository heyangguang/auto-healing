package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	respPkg "github.com/company/auto-healing/internal/pkg/response"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuditHandlerStatsEndpoints(t *testing.T) {
	db := openAuditHandlerStatsTestDB(t)
	createAuditHandlerStatsSchema(t, db)

	tenantID := uuid.New()
	userID := uuid.New()
	createAuditHandlerUser(t, db, userID, "alice")
	insertAuditHandlerLog(t, db, platformmodel.AuditLog{
		ID:           uuid.New(),
		TenantID:     &tenantID,
		UserID:       &userID,
		Username:     "alice",
		Category:     "operation",
		Action:       "update",
		ResourceType: "plugins",
		Status:       "success",
		CreatedAt:    time.Now().UTC(),
	})
	insertAuditHandlerLog(t, db, platformmodel.AuditLog{
		ID:           uuid.New(),
		TenantID:     &tenantID,
		UserID:       &userID,
		Username:     "alice",
		Category:     "operation",
		Action:       "delete",
		ResourceType: "users",
		Status:       "failed",
		CreatedAt:    time.Now().UTC(),
	})

	handler := NewAuditHandlerWithDeps(AuditHandlerDeps{
		Repo:         auditrepo.NewAuditLogRepository(db),
		PlatformRepo: auditrepo.NewPlatformAuditLogRepositoryWithDB(db),
	})
	router := newAuditHandlerStatsRouter(tenantID)
	router.GET("/stats", handler.GetAuditStats)
	router.GET("/users", handler.GetUserRanking)
	router.GET("/actions", handler.GetActionGrouping)

	statsResp := issueAuditStatsRequest(t, router, "/stats")
	if statsResp.Code != respPkg.CodeSuccess {
		t.Fatalf("stats code = %d, want %d", statsResp.Code, respPkg.CodeSuccess)
	}
	stats := decodeAuditStatsMap(t, statsResp.Data)
	if got := int64(stats["total_count"].(float64)); got != 2 {
		t.Fatalf("stats.total_count = %d, want 2", got)
	}

	rankingResp := issueAuditStatsRequest(t, router, "/users?limit=5&days=30")
	if rankingResp.Code != respPkg.CodeSuccess {
		t.Fatalf("ranking code = %d, want %d", rankingResp.Code, respPkg.CodeSuccess)
	}
	rankings := decodeAuditItems(t, rankingResp.Data)
	if len(rankings) != 1 || rankings[0]["username"] != "alice" {
		t.Fatalf("unexpected user rankings: %#v", rankings)
	}

	actionResp := issueAuditStatsRequest(t, router, "/actions?action=delete")
	if actionResp.Code != respPkg.CodeSuccess {
		t.Fatalf("action grouping code = %d, want %d", actionResp.Code, respPkg.CodeSuccess)
	}
	items := decodeAuditItems(t, actionResp.Data)
	if len(items) != 1 || items[0]["action"] != "delete" {
		t.Fatalf("unexpected action grouping: %#v", items)
	}
}

func TestAuditHandlerStatsSupportsTenantVisibleAuthCategory(t *testing.T) {
	db := openAuditHandlerStatsTestDB(t)
	createAuditHandlerStatsSchema(t, db)

	tenantID := uuid.New()
	userID := uuid.New()
	createAuditHandlerUser(t, db, userID, "alice")
	if err := db.Exec(
		"INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)",
		uuid.NewString(),
		userID.String(),
		tenantID.String(),
		uuid.NewString(),
		time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("insert tenant membership: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO platform_audit_logs (
			id, user_id, username, principal_username, subject_scope, subject_tenant_id, subject_tenant_name,
			auth_method, category, action, resource_type, request_path, response_status, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(),
		userID.String(),
		"alice",
		"alice",
		"tenant_user",
		tenantID.String(),
		"Tenant A",
		"password",
		"auth",
		"login",
		"auth",
		"/api/v1/auth/login",
		200,
		"success",
		time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("insert platform auth log: %v", err)
	}

	handler := NewAuditHandlerWithDeps(AuditHandlerDeps{
		Repo:         auditrepo.NewAuditLogRepository(db),
		PlatformRepo: auditrepo.NewPlatformAuditLogRepositoryWithDB(db),
	})
	router := newAuditHandlerStatsRouter(tenantID)
	router.GET("/stats", handler.GetAuditStats)

	statsResp := issueAuditStatsRequest(t, router, "/stats?category=auth")
	if statsResp.Code != respPkg.CodeSuccess {
		t.Fatalf("stats code = %d, want %d", statsResp.Code, respPkg.CodeSuccess)
	}
	stats := decodeAuditStatsMap(t, statsResp.Data)
	if got := int64(stats["total_count"].(float64)); got != 1 {
		t.Fatalf("stats.total_count = %d, want 1", got)
	}
	if got := int64(stats["success_count"].(float64)); got != 1 {
		t.Fatalf("stats.success_count = %d, want 1", got)
	}
}

func TestAuditHandlerTrendSurfacesRepositoryError(t *testing.T) {
	db := openAuditHandlerStatsTestDB(t)
	createAuditHandlerStatsSchema(t, db)

	handler := NewAuditHandlerWithDeps(AuditHandlerDeps{
		Repo:         auditrepo.NewAuditLogRepository(db),
		PlatformRepo: auditrepo.NewPlatformAuditLogRepositoryWithDB(db),
	})
	router := newAuditHandlerStatsRouter(uuid.New())
	router.GET("/trend", handler.GetTrend)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/trend?days=7", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func newAuditHandlerStatsRouter(tenantID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(auditrepo.WithTenantID(context.Background(), tenantID))
		c.Next()
	})
	return router
}

func openAuditHandlerStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "audit-handler.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createAuditHandlerStatsSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT,
			email TEXT,
			password_hash TEXT
		)`,
		`CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY,
			tenant_id TEXT,
			user_id TEXT,
			username TEXT,
			ip_address TEXT,
			user_agent TEXT,
			category TEXT,
			action TEXT,
			resource_type TEXT,
			resource_id TEXT,
			resource_name TEXT,
			request_method TEXT,
			request_path TEXT,
			request_body TEXT,
			response_status INTEGER,
			changes TEXT,
			status TEXT,
			error_message TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE platform_audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			username TEXT,
			principal_username TEXT,
			subject_scope TEXT,
			subject_tenant_id TEXT,
			subject_tenant_name TEXT,
			failure_reason TEXT,
			auth_method TEXT,
			ip_address TEXT,
			user_agent TEXT,
			category TEXT,
			action TEXT,
			resource_type TEXT,
			resource_id TEXT,
			resource_name TEXT,
			request_method TEXT,
			request_path TEXT,
			request_body TEXT,
			response_status INTEGER,
			changes TEXT,
			status TEXT,
			error_message TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE user_tenant_roles (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			tenant_id TEXT,
			role_id TEXT,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create audit schema: %v", err)
		}
	}
}

func createAuditHandlerUser(t *testing.T, db *gorm.DB, id uuid.UUID, username string) {
	t.Helper()
	if err := db.Exec(
		"INSERT INTO users (id, username, email, password_hash) VALUES (?, ?, ?, ?)",
		id.String(),
		username,
		username+"@example.com",
		"hash",
	).Error; err != nil {
		t.Fatalf("create audit user: %v", err)
	}
}

func insertAuditHandlerLog(t *testing.T, db *gorm.DB, log platformmodel.AuditLog) {
	t.Helper()
	if err := db.Create(&log).Error; err != nil {
		t.Fatalf("insert audit log: %v", err)
	}
}

func issueAuditStatsRequest(t *testing.T, router http.Handler, path string) respPkg.Response {
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

func decodeAuditStatsMap(t *testing.T, data any) map[string]any {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal audit stats data: %v", err)
	}
	var items map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		t.Fatalf("unmarshal audit stats data: %v; payload=%s", err, string(payload))
	}
	return items
}

func decodeAuditItems(t *testing.T, data any) []map[string]any {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal audit item data: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		t.Fatalf("unmarshal audit item data: %v; payload=%s", err, string(payload))
	}
	return items
}
