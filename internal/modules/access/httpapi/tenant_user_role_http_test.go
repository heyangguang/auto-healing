package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/jwt"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestValidateInvitationReturnsBadRequestWhenTenantMissing(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createTenantUserRoleHTTPTestSchema(t, db)
	roleID := uuid.New()
	insertRole(t, db, roleID, "viewer", "tenant")
	insertPendingInvitation(t, db, uuid.New(), uuid.New(), roleID, "missing-tenant@example.com", "missing-tenant-token")

	router := newTenantUserRoleHTTPTestRouter(t, db)
	recorder := issueTenantUserRoleRequest(t, router, http.MethodGet, "/api/v1/auth/invitation/missing-tenant-token", nil, nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestRegisterByInvitationReturnsBadRequestWhenTenantMissing(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createTenantUserRoleHTTPTestSchema(t, db)
	roleID := uuid.New()
	insertRole(t, db, roleID, "viewer", "tenant")
	insertPendingInvitation(t, db, uuid.New(), uuid.New(), roleID, "missing-tenant@example.com", "register-missing-tenant-token")

	router := newTenantUserRoleHTTPTestRouter(t, db)
	body := map[string]any{
		"token":        "register-missing-tenant-token",
		"username":     "tenantinvitee",
		"password":     "Tenant123456!",
		"display_name": "Invitee",
	}
	recorder := issueTenantUserRoleJSONRequest(t, router, http.MethodPost, "/api/v1/auth/register", body, nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}

	platformlifecycle.Cleanup()

	var audit struct {
		Username          string
		Status            string
		Category          string
		Action            string
		ResourceType      string
		FailureReason     string
		AuthMethod        string
		SubjectScope      string
		SubjectTenantName string
	}
	if err := db.Table("platform_audit_logs").
		Select("username, status, category, action, resource_type, failure_reason, auth_method, subject_scope, subject_tenant_name").
		Where("request_path = ?", "/api/v1/auth/register").
		Take(&audit).Error; err != nil {
		t.Fatalf("load register audit: %v", err)
	}
	if audit.Username != "tenantinvitee" || audit.Status != "failed" || audit.Category != "auth" || audit.Action != "register" || audit.ResourceType != "auth" {
		t.Fatalf("audit = %+v, want failed auth register audit", audit)
	}
	if audit.FailureReason != authFailureReasonInvitationInvalid || audit.AuthMethod != authMethodInvitationRegister || audit.SubjectScope != authSubjectScopeTenantUser {
		t.Fatalf("audit metadata = %+v, want invitation_invalid/invitation_register/tenant_user", audit)
	}
}

func TestPlatformListMembersReturnsNotFoundWhenTenantMissing(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createTenantUserRoleHTTPTestSchema(t, db)

	router := newTenantUserRoleHTTPTestRouter(t, db)
	recorder := issueTenantUserRoleRequest(t, router, http.MethodGet, "/api/v1/platform/tenants/"+uuid.NewString()+"/members", nil, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestPlatformListInvitationsReturnsNotFoundWhenTenantMissing(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createTenantUserRoleHTTPTestSchema(t, db)

	router := newTenantUserRoleHTTPTestRouter(t, db)
	recorder := issueTenantUserRoleRequest(t, router, http.MethodGet, "/api/v1/platform/tenants/"+uuid.NewString()+"/invitations", nil, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestTenantUserUpdateReturnsForbiddenForGlobalMutation(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createTenantUserRoleHTTPTestSchema(t, db)
	tenantID := uuid.New()
	roleID := uuid.New()
	userID := uuid.New()

	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")
	insertRole(t, db, roleID, "viewer", "tenant")
	insertUser(t, db, userID, "tenant-user", false)
	assignTenantRole(t, db, userID, tenantID, roleID)

	router := newTenantUserRoleHTTPTestRouter(t, db)
	body := map[string]any{"display_name": "Updated Name", "role_id": roleID.String()}
	headers := map[string]string{"X-Test-Tenant-ID": tenantID.String()}
	recorder := issueTenantUserRoleJSONRequest(t, router, http.MethodPut, "/api/v1/tenant/users/"+userID.String(), body, headers)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestTenantUserResetPasswordReturnsForbidden(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createTenantUserRoleHTTPTestSchema(t, db)
	tenantID := uuid.New()
	roleID := uuid.New()
	userID := uuid.New()

	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")
	insertRole(t, db, roleID, "viewer", "tenant")
	insertUser(t, db, userID, "tenant-user", false)
	assignTenantRole(t, db, userID, tenantID, roleID)

	router := newTenantUserRoleHTTPTestRouter(t, db)
	body := map[string]any{"new_password": "Reset123456!"}
	headers := map[string]string{"X-Test-Tenant-ID": tenantID.String()}
	recorder := issueTenantUserRoleJSONRequest(t, router, http.MethodPost, "/api/v1/tenant/users/"+userID.String()+"/reset-password", body, headers)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func createTenantUserRoleHTTPTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	createAuthHandlerSchema(t, db)
	mustExecAuthSQL(t, db, `
		CREATE TABLE tenant_invitations (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT NOT NULL,
			email TEXT NOT NULL,
			role_id TEXT NOT NULL,
			token TEXT,
			token_hash TEXT NOT NULL,
			status TEXT NOT NULL,
			invited_by TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			accepted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
}

func insertPendingInvitation(t *testing.T, db *gorm.DB, tenantID, invitedBy, roleID uuid.UUID, email, token string) {
	t.Helper()
	now := time.Now().UTC()
	expiresAt := now.AddDate(50, 0, 0)
	mustExecAuthSQL(t, db, `
		INSERT INTO tenant_invitations (
			id, tenant_id, email, role_id, token, token_hash, status, invited_by, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), tenantID.String(), email, roleID.String(), token, hashToken(token), "pending", invitedBy.String(), expiresAt, now, now)
}

func newTenantUserRoleHTTPTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = origDB
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "tenant-user-role-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "tenant-user-role-test",
	}, testBlacklistStore{})
	authSvc := authService.NewServiceWithDeps(authService.ServiceDeps{JWTService: jwtSvc, DB: db})
	tenantHandler := NewTenantHandlerWithDeps(TenantHandlerDeps{
		TenantRepo:        accessrepo.NewTenantRepositoryWithDB(db),
		RoleRepo:          accessrepo.NewRoleRepositoryWithDB(db),
		UserRepo:          accessrepo.NewUserRepositoryWithDB(db),
		AuthService:       authSvc,
		InvitationRepo:    accessrepo.NewInvitationRepositoryWithDB(db),
		SettingsRepo:      settingsrepo.NewPlatformSettingsRepositoryWithDB(db),
		PlatformAuditRepo: auditrepo.NewPlatformAuditLogRepositoryWithDB(db),
		EmailService:      &stubInvitationEmailService{},
	})
	tenantUserHandler := NewTenantUserHandlerWithDeps(TenantUserHandlerDeps{
		AuthService: authSvc,
		TenantRepo:  accessrepo.NewTenantRepositoryWithDB(db),
		UserRepo:    accessrepo.NewUserRepositoryWithDB(db),
		RoleRepo:    accessrepo.NewRoleRepositoryWithDB(db),
	})

	api := router.Group("/api/v1")
	api.GET("/auth/invitation/:token", tenantHandler.ValidateInvitation)
	api.POST("/auth/register", tenantHandler.RegisterByInvitation)
	api.GET("/platform/tenants/:id/members", tenantHandler.ListMembers)
	api.GET("/platform/tenants/:id/invitations", tenantHandler.ListInvitations)

	tenant := api.Group("/tenant")
	tenant.Use(testTenantContextMiddleware())
	tenant.PUT("/users/:id", tenantUserHandler.UpdateTenantUser)
	tenant.POST("/users/:id/reset-password", tenantUserHandler.ResetTenantUserPassword)
	return router
}

func testTenantContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawTenantID := c.GetHeader("X-Test-Tenant-ID")
		if rawTenantID == "" {
			c.Next()
			return
		}
		tenantID, err := uuid.Parse(rawTenantID)
		if err != nil {
			c.Next()
			return
		}
		c.Request = c.Request.WithContext(platformrepo.WithTenantID(c.Request.Context(), tenantID))
		c.Next()
	}
}

func issueTenantUserRoleRequest(t *testing.T, router *gin.Engine, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	requestBody := bytes.NewReader(body)
	if body == nil {
		requestBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, requestBody)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func issueTenantUserRoleJSONRequest(t *testing.T, router *gin.Engine, method, path string, payload any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return issueTenantUserRoleRequest(t, router, method, path, body, headers)
}
