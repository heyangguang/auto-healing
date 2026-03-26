package handler

import (
	"net/http"
	"testing"

	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/google/uuid"
)

func TestSetupAuthRoutesAuthMeUsesCurrentTenantContext(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	tenantAID := uuid.New()
	tenantBID := uuid.New()
	userID := uuid.New()
	roleAID := uuid.New()
	roleBID := uuid.New()
	permAID := uuid.New()
	permBID := uuid.New()

	insertTenant(t, db, tenantAID, "Tenant A", "tenant-a")
	insertTenant(t, db, tenantBID, "Tenant B", "tenant-b")
	insertUser(t, db, userID, "tenant-user", false)
	insertRole(t, db, roleAID, "tenant_a_operator", "tenant")
	insertRole(t, db, roleBID, "tenant_b_operator", "tenant")
	insertPermission(t, db, permAID, "task:list")
	insertPermission(t, db, permBID, "playbook:list")
	attachPermissionToRole(t, db, roleAID, permAID)
	attachPermissionToRole(t, db, roleBID, permBID)
	assignTenantRole(t, db, userID, tenantAID, roleAID)
	assignTenantRole(t, db, userID, tenantBID, roleBID)

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "tenant-user", nil, nil, func(claims *jwt.Claims) {
		claims.TenantIDs = []string{tenantAID.String(), tenantBID.String()}
		claims.DefaultTenantID = tenantAID.String()
	})

	resp := issueAuthMe(t, router, token, nil)
	if len(resp.Data.Permissions) != 1 || resp.Data.Permissions[0] != "task:list" {
		t.Fatalf("permissions = %v, want [task:list]", resp.Data.Permissions)
	}
	if len(resp.Data.Roles) != 1 || resp.Data.Roles[0] != "tenant_a_operator" {
		t.Fatalf("roles = %v, want [tenant_a_operator]", resp.Data.Roles)
	}
}

func TestSetupAuthRoutesAuthMeUsesImpersonationContext(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	tenantID := uuid.New()
	userID := uuid.New()
	requestID := uuid.New()
	platformRoleID := uuid.New()
	impRoleID := uuid.New()
	platformPermID := uuid.New()
	impPermID := uuid.New()

	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")
	insertUser(t, db, userID, "platform-admin", true)
	insertRole(t, db, platformRoleID, "platform_admin", "platform")
	insertRole(t, db, impRoleID, "impersonation_accessor", "tenant")
	insertPermission(t, db, platformPermID, "platform:tenants:manage")
	insertPermission(t, db, impPermID, "plugin:list")
	attachPermissionToRole(t, db, platformRoleID, platformPermID)
	attachPermissionToRole(t, db, impRoleID, impPermID)
	assignPlatformRole(t, db, userID, platformRoleID)
	insertImpersonationRequest(t, db, requestID, userID, tenantID, "platform-admin", "Tenant A")

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "platform-admin", []string{"platform_admin"}, []string{"platform:tenants:manage"}, func(claims *jwt.Claims) {
		claims.IsPlatformAdmin = true
	})

	resp := issueAuthMe(t, router, token, map[string]string{
		"X-Impersonation":            "true",
		"X-Impersonation-Request-ID": requestID.String(),
		"X-Tenant-ID":                tenantID.String(),
	})
	if len(resp.Data.Permissions) != 1 || resp.Data.Permissions[0] != "plugin:list" {
		t.Fatalf("permissions = %v, want [plugin:list]", resp.Data.Permissions)
	}
	if !resp.Data.IsPlatformAdmin {
		t.Fatalf("is_platform_admin = false, want true")
	}
}

func TestSetupAuthRoutesAuthMeReturnsInternalErrorWhenPermissionQueryFails(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	tenantID := uuid.New()
	userID := uuid.New()
	roleID := uuid.New()
	permID := uuid.New()

	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")
	insertUser(t, db, userID, "tenant-user", false)
	insertRole(t, db, roleID, "tenant_operator", "tenant")
	insertPermission(t, db, permID, "task:list")
	attachPermissionToRole(t, db, roleID, permID)
	assignTenantRole(t, db, userID, tenantID, roleID)
	mustExecAuthSQL(t, db, `DROP TABLE permissions;`)

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "tenant-user", nil, nil, func(claims *jwt.Claims) {
		claims.TenantIDs = []string{tenantID.String()}
		claims.DefaultTenantID = tenantID.String()
	})

	recorder := issueAuthRequest(t, router, http.MethodGet, "/api/v1/auth/me", token, nil, nil)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
}

func TestSetupAuthRoutesAuthMeRejectsStaleDefaultTenant(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	tenantAID := uuid.New()
	tenantBID := uuid.New()
	userID := uuid.New()
	roleAID := uuid.New()
	permAID := uuid.New()

	insertTenant(t, db, tenantAID, "Tenant A", "tenant-a")
	insertTenant(t, db, tenantBID, "Tenant B", "tenant-b")
	insertUser(t, db, userID, "tenant-user", false)
	insertRole(t, db, roleAID, "tenant_a_operator", "tenant")
	insertPermission(t, db, permAID, "task:list")
	attachPermissionToRole(t, db, roleAID, permAID)
	assignTenantRole(t, db, userID, tenantAID, roleAID)

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "tenant-user", nil, nil, func(claims *jwt.Claims) {
		claims.TenantIDs = []string{tenantAID.String(), tenantBID.String()}
		claims.DefaultTenantID = tenantBID.String()
	})

	recorder := issueAuthRequest(t, router, http.MethodGet, "/api/v1/auth/me", token, nil, nil)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestSetupAuthRoutesProfileActivitiesRejectsStaleDefaultTenant(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	tenantAID := uuid.New()
	tenantBID := uuid.New()
	userID := uuid.New()
	roleAID := uuid.New()

	insertTenant(t, db, tenantAID, "Tenant A", "tenant-a")
	insertTenant(t, db, tenantBID, "Tenant B", "tenant-b")
	insertUser(t, db, userID, "tenant-user", false)
	insertRole(t, db, roleAID, "tenant_a_operator", "tenant")
	assignTenantRole(t, db, userID, tenantAID, roleAID)

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "tenant-user", nil, nil, func(claims *jwt.Claims) {
		claims.TenantIDs = []string{tenantAID.String(), tenantBID.String()}
		claims.DefaultTenantID = tenantBID.String()
	})

	recorder := issueAuthRequest(t, router, http.MethodGet, "/api/v1/auth/profile/activities", token, nil, nil)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}
