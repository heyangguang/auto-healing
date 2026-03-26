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
	mustExecMiddlewareSQL(t, db, `CREATE TABLE user_tenant_roles (id TEXT PRIMARY KEY NOT NULL, user_id TEXT, tenant_id TEXT, role_id TEXT, created_at DATETIME);`)
	tenantID := uuid.New()
	userID := uuid.New()
	mustExecMiddlewareSQL(t, db, `
		INSERT INTO tenants (id, name, code, description, icon, status, created_at, updated_at)
		VALUES (?, 'Tenant A', 'tenant-a', '', '', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, tenantID.String())
	mustExecMiddlewareSQL(t, db, `
		INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, uuid.NewString(), userID.String(), tenantID.String(), uuid.NewString())
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	router := gin.New()
	router.GET("/common", func(c *gin.Context) {
		c.Set(UserIDKey, userID.String())
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

func TestEnsureActiveTenantReturnsInternalErrorWhenLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newMiddlewareTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/tenant", nil)

	if ensureActiveTenant(c, uuid.New()) {
		t.Fatal("ensureActiveTenant() = true, want false")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var resp middlewareErrorResponse
	if err := decodeMiddlewareError(recorder, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodeTenantLookupFailed {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeTenantLookupFailed)
	}
}

func TestCommonTenantMiddlewareReturnsInternalErrorWhenPlatformPermissionReloadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newMiddlewareTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	router := gin.New()
	router.GET("/common", func(c *gin.Context) {
		c.Set(UserIDKey, uuid.NewString())
		c.Set(IsPlatformAdminKey, true)
		c.Set(PermissionsKey, []string{"stale:perm"})
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
	if resp.ErrorCode != ErrorCodePlatformPermissionReloadFailed {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodePlatformPermissionReloadFailed)
	}
}

func TestTenantMiddlewareReturnsInternalErrorWhenPermissionReloadFails(t *testing.T) {
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
	mustExecMiddlewareSQL(t, db, `
		INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, uuid.NewString(), userID.String(), tenantID.String(), uuid.NewString())
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	router := gin.New()
	router.GET("/tenant", func(c *gin.Context) {
		c.Set(UserIDKey, userID.String())
		c.Set(DefaultTenantIDKey, tenantID.String())
		c.Set(TenantIDsKey, []string{tenantID.String()})
		c.Set(PermissionsKey, []string{"old:perm"})
	}, TenantMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/tenant", nil)
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

func TestResolveRegularTenantRejectsInvalidDefaultTenantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/tenant", nil)

	if _, ok := resolveRegularTenant(c, nil, "not-a-uuid"); ok {
		t.Fatal("resolveRegularTenant() ok = true, want false")
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

	if _, ok := resolveRegularTenant(c, nil, ""); ok {
		t.Fatal("resolveRegularTenant() ok = true, want false")
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

	if _, ok := resolveRegularTenant(c, []string{tenantID.String()}, tenantID.String()); ok {
		t.Fatal("resolveRegularTenant() ok = true, want false")
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

	if _, ok := resolveCommonRouteTenant(c); ok {
		t.Fatal("resolveCommonRouteTenant() ok = true, want false")
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
