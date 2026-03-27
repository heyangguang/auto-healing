package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type testBlacklistStore struct{}

func (testBlacklistStore) Add(context.Context, string, time.Time) error { return nil }

func (testBlacklistStore) Exists(context.Context, string) (bool, error) { return false, nil }

func attachPermissionToRole(t *testing.T, db *gorm.DB, roleID, permissionID uuid.UUID) {
	t.Helper()
	mustExecAuthSQL(t, db, `
		INSERT INTO role_permissions (id, role_id, permission_id, created_at)
		VALUES (?, ?, ?, ?)
	`, uuid.NewString(), roleID.String(), permissionID.String(), time.Now().UTC())
}

func assignPlatformRole(t *testing.T, db *gorm.DB, userID, roleID uuid.UUID) {
	t.Helper()
	mustExecAuthSQL(t, db, `
		INSERT INTO user_platform_roles (id, user_id, role_id, created_at)
		VALUES (?, ?, ?, ?)
	`, uuid.NewString(), userID.String(), roleID.String(), time.Now().UTC())
}

func assignTenantRole(t *testing.T, db *gorm.DB, userID, tenantID, roleID uuid.UUID) {
	t.Helper()
	mustExecAuthSQL(t, db, `
		INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.NewString(), userID.String(), tenantID.String(), roleID.String(), time.Now().UTC())
}

func insertTenantMembership(t *testing.T, db *gorm.DB, userID, tenantID uuid.UUID) {
	t.Helper()
	mustExecAuthSQL(t, db, `
		INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.NewString(), userID.String(), tenantID.String(), uuid.NewString(), time.Now().UTC())
}

func insertImpersonationRequest(t *testing.T, db *gorm.DB, requestID, requesterID, tenantID uuid.UUID, requesterName, tenantName string) {
	t.Helper()
	now := time.Now().UTC()
	expiresAt := now.Add(time.Hour)
	mustExecAuthSQL(t, db, `
		INSERT INTO impersonation_requests (
			id, requester_id, requester_name, tenant_id, tenant_name, reason, duration_minutes, status,
			session_started_at, session_expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 'test', 60, ?, ?, ?, ?, ?)
	`, requestID.String(), requesterID.String(), requesterName, tenantID.String(), tenantName, model.ImpersonationStatusActive, now, expiresAt, now, now)
}

func newAuthHandlerTestRouter(t *testing.T, db *gorm.DB) (*gin.Engine, *jwt.Service) {
	t.Helper()
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "handler-test",
	}, testBlacklistStore{})
	return newAuthHandlerTestRouterWithJWTService(t, db, jwtSvc), jwtSvc
}

func newAuthHandlerTestRouterWithJWTService(t *testing.T, db *gorm.DB, jwtSvc *jwt.Service) *gin.Engine {
	t.Helper()
	platformlifecycle.Cleanup()
	t.Cleanup(platformlifecycle.Cleanup)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = origDB
	})

	gin.SetMode(gin.TestMode)
	logger.Init(&config.LogConfig{})
	router := gin.New()
	authHandler := &AuthHandler{
		authSvc:           authService.NewService(jwtSvc),
		jwtSvc:            jwtSvc,
		auditRepo:         repository.NewAuditLogRepository(db),
		platformAuditRepo: repository.NewPlatformAuditLogRepository(),
		userRepo:          repository.NewUserRepository(),
	}

	api := router.Group("/api/v1")
	registerAuthTestRoutes(api, authHandler)
	return router
}

func registerAuthTestRoutes(api *gin.RouterGroup, authHandler *AuthHandler) {
	auth := api.Group("/auth")
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.RefreshToken)
	auth.GET("/invitation/:token", ValidateInvitation)
	auth.POST("/register", RegisterByInvitation(authHandler.GetAuthService()))

	authProtected := auth.Group("")
	authProtected.Use(middleware.JWTAuth(authHandler.GetJWTService()))
	authProtected.GET("/me",
		middleware.ImpersonationMiddleware(),
		RequireAuthTenantContext(),
		authHandler.GetCurrentUser,
	)
	authProtected.GET("/profile", authHandler.GetProfile)
	authProtected.GET("/profile/login-history", authHandler.GetLoginHistory)
	authProtected.GET("/profile/activities",
		middleware.ImpersonationMiddleware(),
		RequireAuthTenantContext(),
		authHandler.GetProfileActivities,
	)

	authAudited := authProtected.Group("")
	authAudited.Use(middleware.ImpersonationMiddleware())
	authAudited.Use(OptionalAuthTenantContext())
	authAudited.Use(middleware.AuditMiddleware())
	authAudited.PUT("/profile", authHandler.UpdateProfile)
	authAudited.PUT("/password", authHandler.ChangePassword)
	authAudited.POST("/logout", authHandler.Logout)
}

func issueAuthMe(t *testing.T, router *gin.Engine, token string, headers map[string]string) authMeResponse {
	t.Helper()
	recorder := issueAuthRequest(t, router, http.MethodGet, "/api/v1/auth/me", token, headers, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var payload authMeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

func issueAuthRequest(t *testing.T, router *gin.Engine, method, path, token string, headers map[string]string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	reqBody := bytes.NewReader(body)
	if body == nil {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func mustAccessToken(t *testing.T, jwtSvc *jwt.Service, userID uuid.UUID, username string, roles, permissions []string, opts ...func(*jwt.Claims)) string {
	t.Helper()
	pair, err := jwtSvc.GenerateTokenPair(userID.String(), username, roles, permissions, opts...)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return pair.AccessToken
}
