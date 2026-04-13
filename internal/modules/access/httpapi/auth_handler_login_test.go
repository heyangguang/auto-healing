package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/company/auto-healing/internal/pkg/response"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestIsLoginUnauthorizedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "invalid credentials", err: authService.ErrInvalidCredentials, want: true},
		{name: "locked", err: authService.ErrUserLocked, want: true},
		{name: "inactive", err: authService.ErrUserInactive, want: true},
		{name: "generic", err: errors.New("db down"), want: false},
	}

	for _, tc := range cases {
		if got := isLoginUnauthorizedError(tc.err); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestLoginFailureStatusCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid credentials", err: authService.ErrInvalidCredentials, want: http.StatusUnauthorized},
		{name: "locked", err: authService.ErrUserLocked, want: http.StatusUnauthorized},
		{name: "infra", err: errors.New("db down"), want: http.StatusInternalServerError},
	}

	for _, tc := range cases {
		if got := loginFailureStatusCode(tc.err); got != tc.want {
			t.Fatalf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestLoginAuditErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "invalid credentials", err: authService.ErrInvalidCredentials, want: authService.ErrInvalidCredentials.Error()},
		{name: "locked", err: authService.ErrUserLocked, want: authService.ErrUserLocked.Error()},
		{name: "infra", err: errors.New("db down"), want: "登录失败，请稍后重试"},
	}

	for _, tc := range cases {
		if got := loginAuditErrorMessage(tc.err); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeLoginHistoryErrorMessage(t *testing.T) {
	unauthorized := http.StatusUnauthorized
	internal := http.StatusInternalServerError

	cases := []struct {
		name    string
		status  string
		code    *int
		message string
		want    string
	}{
		{name: "success hides message", status: "success", code: &unauthorized, message: "ignored", want: ""},
		{name: "unauthorized keeps business message", status: "failed", code: &unauthorized, message: authService.ErrInvalidCredentials.Error(), want: authService.ErrInvalidCredentials.Error()},
		{name: "internal hides raw message", status: "failed", code: &internal, message: "sql: connection refused", want: "登录失败，请稍后重试"},
	}

	for _, tc := range cases {
		if got := sanitizeLoginHistoryErrorMessage(tc.status, tc.code, tc.message); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestSetupAuthRoutesLoginReturnsTokensAndCurrentTenant(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	tenantID := uuid.New()
	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")
	insertLoginUser(t, db, userID, "login-user", "correct-password")
	insertTenantMembership(t, db, userID, tenantID)

	router, _ := newAuthHandlerTestRouter(t, db)
	recorder := issueLoginRequest(t, router, "login-user", "correct-password")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var resp struct {
		Code    int                       `json:"code"`
		Message string                    `json:"message"`
		Data    authService.LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != response.CodeSuccess {
		t.Fatalf("code = %d, want %d", resp.Code, response.CodeSuccess)
	}
	if resp.Message != "success" {
		t.Fatalf("message = %q, want %q", resp.Message, "success")
	}
	if resp.Data.AccessToken == "" || resp.Data.RefreshToken == "" {
		t.Fatalf("tokens = (%q, %q), want both non-empty", resp.Data.AccessToken, resp.Data.RefreshToken)
	}
	if resp.Data.User.Username != "login-user" {
		t.Fatalf("username = %q, want %q", resp.Data.User.Username, "login-user")
	}
	if resp.Data.CurrentTenantID != tenantID.String() {
		t.Fatalf("current_tenant_id = %q, want %q", resp.Data.CurrentTenantID, tenantID.String())
	}
	if len(resp.Data.Tenants) != 1 || resp.Data.Tenants[0].ID != tenantID.String() {
		t.Fatalf("tenants = %+v, want tenant %s", resp.Data.Tenants, tenantID)
	}
}

func TestSetupAuthRoutesLoginReturnsUnauthorizedForInvalidCredentials(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	insertLoginUser(t, db, uuid.New(), "login-user", "correct-password")

	router, _ := newAuthHandlerTestRouter(t, db)
	recorder := issueLoginRequest(t, router, "login-user", "wrong-password")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Message != authService.ErrInvalidCredentials.Error() {
		t.Fatalf("message = %q, want %q", resp.Message, authService.ErrInvalidCredentials.Error())
	}
}

func TestSetupAuthRoutesLoginWritesUnknownUsernameFailureToPlatformAudit(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	router, _ := newAuthHandlerTestRouter(t, db)
	recorder := issueLoginRequest(t, router, "ghost-user", "wrong-password")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}

	platformlifecycle.Cleanup()

	var audit struct {
		Username string
		Status   string
		Action   string
		UserID   *string
	}
	if err := db.Table("platform_audit_logs").
		Select("username, status, action, user_id").
		Where("username = ?", "ghost-user").
		Take(&audit).Error; err != nil {
		t.Fatalf("load platform audit: %v", err)
	}
	if audit.Status != "failed" || audit.Action != "login" {
		t.Fatalf("audit = %+v, want failed login", audit)
	}
	if audit.UserID != nil {
		t.Fatalf("user_id = %v, want nil for unknown username", *audit.UserID)
	}

	var tenantAuditCount int64
	if err := db.Table("audit_logs").Count(&tenantAuditCount).Error; err != nil {
		t.Fatalf("count tenant audits: %v", err)
	}
	if tenantAuditCount != 0 {
		t.Fatalf("tenant audit count = %d, want 0", tenantAuditCount)
	}
}

func TestSetupAuthRoutesLoginWritesTenantUserSuccessToPlatformAudit(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	tenantID := uuid.New()
	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")
	insertLoginUser(t, db, userID, "login-user", "correct-password")
	insertTenantMembership(t, db, userID, tenantID)

	router, _ := newAuthHandlerTestRouter(t, db)
	recorder := issueLoginRequest(t, router, "login-user", "correct-password")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	platformlifecycle.Cleanup()

	var audit struct {
		Username string
		Status   string
		Action   string
		UserID   string
	}
	if err := db.Table("platform_audit_logs").
		Select("username, status, action, user_id").
		Where("username = ?", "login-user").
		Take(&audit).Error; err != nil {
		t.Fatalf("load platform audit: %v", err)
	}
	if audit.Status != "success" || audit.Action != "login" || audit.UserID != userID.String() {
		t.Fatalf("audit = %+v, want successful platform login audit for %s", audit, userID)
	}

	var tenantAuditCount int64
	if err := db.Table("audit_logs").Count(&tenantAuditCount).Error; err != nil {
		t.Fatalf("count tenant audits: %v", err)
	}
	if tenantAuditCount != 0 {
		t.Fatalf("tenant audit count = %d, want 0", tenantAuditCount)
	}
}

func TestSetupAuthRoutesLoginReturnsInternalErrorWithoutLeakingDetails(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	insertLoginUser(t, db, uuid.New(), "login-user", "correct-password")
	mustExecAuthSQL(t, db, `DROP TABLE permissions;`)

	router, _ := newAuthHandlerTestRouter(t, db)
	recorder := issueLoginRequest(t, router, "login-user", "correct-password")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Message != "登录失败" {
		t.Fatalf("message = %q, want %q", resp.Message, "登录失败")
	}
	if strings.Contains(recorder.Body.String(), "no such table") {
		t.Fatalf("body leaked internal details: %s", recorder.Body.String())
	}
}

func issueLoginRequest(t *testing.T, router http.Handler, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func insertLoginUser(t *testing.T, db *gorm.DB, userID uuid.UUID, username, password string) {
	t.Helper()
	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now().UTC()
	mustExecAuthSQL(t, db, `
		INSERT INTO users (
			id, username, email, password_hash, display_name, status, password_changed_at, failed_login_count,
			created_at, updated_at, is_platform_admin
		) VALUES (?, ?, ?, ?, ?, 'active', ?, 0, ?, ?, ?)
	`, userID.String(), username, username+"@example.com", passwordHash, username, now, now, now, false)
}
