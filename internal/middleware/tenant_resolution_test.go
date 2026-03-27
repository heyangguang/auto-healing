package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/database"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestResolveRegularTenantRejectsInvalidDefaultTenantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/tenant", nil)
	tenantRepo := accessrepo.NewTenantRepositoryWithDB(newMiddlewareTestDB(t))

	if _, ok := resolveRegularTenantWithRepo(c, tenantRepo, nil, "not-a-uuid"); ok {
		t.Fatal("resolveRegularTenantWithRepo() ok = true, want false")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	var resp middlewareErrorResponse
	if err := decodeMiddlewareError(recorder, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodeDefaultTenantInvalid {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeDefaultTenantInvalid)
	}
}

func TestResolveRegularTenantReturnsInternalErrorWhenMembershipLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newMiddlewareTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	tenantID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/tenant", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	c.Request = req
	c.Set(UserIDKey, uuid.NewString())
	tenantRepo := accessrepo.NewTenantRepositoryWithDB(db)

	if _, ok := resolveRegularTenantWithRepo(c, tenantRepo, nil, ""); ok {
		t.Fatal("resolveRegularTenantWithRepo() ok = true, want false")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var resp middlewareErrorResponse
	if err := decodeMiddlewareError(recorder, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodeTenantMembershipLookupFailed {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeTenantMembershipLookupFailed)
	}
}

func TestResolveRegularTenantRejectsDefaultTenantWhenMembershipRevoked(t *testing.T) {
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
	mustExecMiddlewareSQL(t, db, `CREATE TABLE user_tenant_roles (id TEXT PRIMARY KEY NOT NULL, user_id TEXT, tenant_id TEXT, role_id TEXT, created_at DATETIME);`)
	tenantID := uuid.New()
	userID := uuid.New()
	mustExecMiddlewareSQL(t, db, `
		INSERT INTO tenants (id, name, code, description, icon, status, created_at, updated_at)
		VALUES (?, 'Tenant A', 'tenant-a', '', '', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, tenantID.String())
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/tenant", nil)
	c.Set(UserIDKey, userID.String())
	tenantRepo := accessrepo.NewTenantRepositoryWithDB(db)

	if _, ok := resolveRegularTenantWithRepo(c, tenantRepo, []string{tenantID.String()}, tenantID.String()); ok {
		t.Fatal("resolveRegularTenantWithRepo() ok = true, want false")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	var resp middlewareErrorResponse
	if err := decodeMiddlewareError(recorder, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodeTenantAccessDenied {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeTenantAccessDenied)
	}
}

func TestResolveCommonRouteTenantRejectsDefaultTenantWhenMembershipRevoked(t *testing.T) {
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
	mustExecMiddlewareSQL(t, db, `CREATE TABLE user_tenant_roles (id TEXT PRIMARY KEY NOT NULL, user_id TEXT, tenant_id TEXT, role_id TEXT, created_at DATETIME);`)
	tenantID := uuid.New()
	userID := uuid.New()
	mustExecMiddlewareSQL(t, db, `
		INSERT INTO tenants (id, name, code, description, icon, status, created_at, updated_at)
		VALUES (?, 'Tenant A', 'tenant-a', '', '', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, tenantID.String())
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/common", nil)
	c.Set(UserIDKey, userID.String())
	c.Set(DefaultTenantIDKey, tenantID.String())
	c.Set(TenantIDsKey, []string{tenantID.String()})
	tenantRepo := accessrepo.NewTenantRepositoryWithDB(db)

	if _, ok := resolveCommonRouteTenantWithRepo(c, tenantRepo); ok {
		t.Fatal("resolveCommonRouteTenantWithRepo() ok = true, want false")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	var resp middlewareErrorResponse
	if err := decodeMiddlewareError(recorder, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodeTenantAccessDenied {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeTenantAccessDenied)
	}
}
