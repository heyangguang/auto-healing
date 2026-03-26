package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestGetTenantUUIDReturnsNilWithoutTenantContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	if got := GetTenantUUID(c); got != uuid.Nil {
		t.Fatalf("GetTenantUUID() = %v, want %v", got, uuid.Nil)
	}
}

func TestGetTenantUUIDReadsInjectedTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	expected := uuid.New()
	c.Set(TenantIDKey, expected.String())

	if got := GetTenantUUID(c); got != expected {
		t.Fatalf("GetTenantUUID() = %v, want %v", got, expected)
	}
}

func TestCommonTenantMiddlewareReturnsInternalErrorWhenPermissionReloadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newMiddlewareTestDB(t)
	mustExecMiddlewareSQL(t, db, `
		CREATE TABLE tenants (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			icon TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	tenantID := uuid.New()
	mustExecMiddlewareSQL(t, db, `
		INSERT INTO tenants (id, name, code, description, icon, status, created_at, updated_at)
		VALUES (?, 'Tenant A', 'tenant-a', '', '', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, tenantID.String())
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	router := gin.New()
	router.GET("/common", func(c *gin.Context) {
		c.Set(UserIDKey, uuid.NewString())
		c.Set(DefaultTenantIDKey, tenantID.String())
		c.Set(TenantIDsKey, []string{tenantID.String()})
		c.Set(PermissionsKey, []string{"old:perm"})
	}, CommonTenantMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/common", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var resp middlewareErrorResponse
	if err := decodeMiddlewareError(recorder, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodeTenantPermissionReloadFailed {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeTenantPermissionReloadFailed)
	}
}
