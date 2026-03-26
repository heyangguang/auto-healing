package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/jwt"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type authItemsResponse[T any] struct {
	Code int `json:"code"`
	Data struct {
		Items []T `json:"items"`
	} `json:"data"`
}

func TestSetupAuthRoutesProfileLoginHistoryUsesCurrentTenantClaim(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	tenantA := uuid.New()
	tenantB := uuid.New()
	insertUser(t, db, userID, "tenant-user", false)
	insertTenant(t, db, tenantA, "Tenant A", "tenant-a")
	insertTenant(t, db, tenantB, "Tenant B", "tenant-b")
	insertTenantMembership(t, db, userID, tenantA)
	insertTenantMembership(t, db, userID, tenantB)
	insertAuthAuditLog(t, db, tenantA, userID, "login-a", "login")
	insertAuthAuditLog(t, db, tenantB, userID, "login-b", "login")

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "tenant-user", nil, nil, func(claims *jwt.Claims) {
		claims.TenantIDs = []string{tenantA.String(), tenantB.String()}
		claims.DefaultTenantID = tenantA.String()
	})

	recorder := issueAuthRequest(t, router, http.MethodGet, "/api/v1/auth/profile/login-history", token, map[string]string{
		"X-Tenant-ID": tenantB.String(),
	}, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var resp authItemsResponse[LoginHistoryItem]
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].Action != "login-b" {
		t.Fatalf("items = %+v, want tenant-b history only", resp.Data.Items)
	}
}

func TestSetupAuthRoutesProfileActivitiesUsesCurrentTenantClaim(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	tenantA := uuid.New()
	tenantB := uuid.New()
	insertUser(t, db, userID, "tenant-user", false)
	insertTenant(t, db, tenantA, "Tenant A", "tenant-a")
	insertTenant(t, db, tenantB, "Tenant B", "tenant-b")
	insertTenantMembership(t, db, userID, tenantA)
	insertTenantMembership(t, db, userID, tenantB)
	insertAuthAuditLog(t, db, tenantA, userID, "op-a", "operation")
	insertAuthAuditLog(t, db, tenantB, userID, "op-b", "operation")

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "tenant-user", nil, nil, func(claims *jwt.Claims) {
		claims.TenantIDs = []string{tenantA.String(), tenantB.String()}
		claims.DefaultTenantID = tenantA.String()
	})

	recorder := issueAuthRequest(t, router, http.MethodGet, "/api/v1/auth/profile/activities", token, map[string]string{
		"X-Tenant-ID": tenantB.String(),
	}, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var resp authItemsResponse[ProfileActivityItem]
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].Action != "op-b" {
		t.Fatalf("items = %+v, want tenant-b activities only", resp.Data.Items)
	}
}

func TestSetupAuthRoutesProfileActivitiesReturnsForbiddenWithoutTenantContext(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	insertUser(t, db, userID, "tenant-user", false)

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "tenant-user", nil, nil)

	recorder := issueAuthRequest(t, router, http.MethodGet, "/api/v1/auth/profile/activities", token, nil, nil)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestSetupAuthRoutesProfileLoginHistoryReturnsForbiddenForDisabledTenant(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	tenantID := uuid.New()
	insertUser(t, db, userID, "tenant-user", false)
	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")
	insertTenantMembership(t, db, userID, tenantID)
	mustExecAuthSQL(t, db, `UPDATE tenants SET status = 'disabled' WHERE id = ?`, tenantID.String())

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "tenant-user", nil, nil, func(claims *jwt.Claims) {
		claims.TenantIDs = []string{tenantID.String()}
		claims.DefaultTenantID = tenantID.String()
	})

	recorder := issueAuthRequest(t, router, http.MethodGet, "/api/v1/auth/profile/login-history", token, nil, nil)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestSetupAuthRoutesProfileActivitiesReturnsForbiddenForRevokedTenantMembership(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	tenantID := uuid.New()
	insertUser(t, db, userID, "tenant-user", false)
	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")

	router, jwtSvc := newAuthHandlerTestRouter(t, db)
	token := mustAccessToken(t, jwtSvc, userID, "tenant-user", nil, nil, func(claims *jwt.Claims) {
		claims.TenantIDs = []string{tenantID.String()}
		claims.DefaultTenantID = tenantID.String()
	})

	recorder := issueAuthRequest(t, router, http.MethodGet, "/api/v1/auth/profile/activities", token, nil, nil)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestChangePasswordReturnsInternalErrorOnRepositoryFailure(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/auth/password", strings.NewReader(`{"old_password":"old","new_password":"new-password"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.UserIDKey, uuid.NewString())

	handler := &AuthHandler{authSvc: authService.NewService(nil)}
	handler.ChangePassword(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
}

func TestUpdateProfileReturnsInternalErrorOnRepositoryFailure(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", strings.NewReader(`{"display_name":"updated"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.UserIDKey, uuid.NewString())

	handler := &AuthHandler{authSvc: authService.NewService(nil)}
	handler.UpdateProfile(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
}

func insertAuthAuditLog(t *testing.T, db *gorm.DB, tenantID, userID uuid.UUID, action, category string) {
	t.Helper()
	now := time.Now().UTC()
	mustExecAuthSQL(t, db, `
		INSERT INTO audit_logs (
			id, tenant_id, user_id, username, ip_address, user_agent, category, action, resource_type,
			request_method, request_path, response_status, status, created_at
		) VALUES (?, ?, ?, ?, '127.0.0.1', 'test-agent', ?, ?, 'auth', 'GET', '/api/v1/auth/test', 200, 'success', ?)
	`, uuid.NewString(), tenantID.String(), userID.String(), "tenant-user", category, action, now)
}
