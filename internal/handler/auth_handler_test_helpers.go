package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = origDB
	})

	gin.SetMode(gin.TestMode)
	logger.Init(&config.LogConfig{})
	router := gin.New()
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "handler-test",
	}, nil)
	handlers := &Handlers{
		Auth: &AuthHandler{
			authSvc:           authService.NewService(jwtSvc),
			jwtSvc:            jwtSvc,
			auditRepo:         repository.NewAuditLogRepository(db),
			platformAuditRepo: repository.NewPlatformAuditLogRepository(),
			userRepo:          repository.NewUserRepository(),
		},
	}

	api := router.Group("/api/v1")
	setupAuthRoutes(api, handlers)
	return router, jwtSvc
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
