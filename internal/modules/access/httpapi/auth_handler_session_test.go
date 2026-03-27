package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/gin-gonic/gin"
	golangjwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type logoutBlacklistRecorder struct {
	added []string
	items map[string]time.Time
}

type failingLogoutBlacklistRecorder struct {
	added      []string
	failOnCall int
	callCount  int
}

func (r *logoutBlacklistRecorder) Add(_ context.Context, jti string, exp time.Time) error {
	r.added = append(r.added, jti)
	if r.items == nil {
		r.items = make(map[string]time.Time)
	}
	r.items[jti] = exp
	return nil
}

func (r *logoutBlacklistRecorder) Exists(_ context.Context, jti string) (bool, error) {
	exp, ok := r.items[jti]
	if !ok {
		return false, nil
	}
	return exp.After(time.Now()), nil
}

func (r *failingLogoutBlacklistRecorder) Add(_ context.Context, jti string, _ time.Time) error {
	r.callCount++
	if r.callCount == r.failOnCall {
		return errors.New("blacklist down")
	}
	r.added = append(r.added, jti)
	return nil
}

func (r *failingLogoutBlacklistRecorder) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func TestRevokeRefreshTokenRejectsMismatchedUser(t *testing.T) {
	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	handler := &AuthHandler{
		jwtSvc:  jwtSvc,
		authSvc: authService.NewService(jwtSvc),
	}

	pair, err := jwtSvc.GenerateTokenPair("user-a", "user-a", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	err = handler.revokeRefreshToken(context.Background(), pair.RefreshToken, "user-b")
	if err != errLogoutRefreshTokenUserMismatch {
		t.Fatalf("revokeRefreshToken() error = %v, want %v", err, errLogoutRefreshTokenUserMismatch)
	}
}

func TestRevokeRefreshTokenBlacklistsMatchingToken(t *testing.T) {
	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	handler := &AuthHandler{
		jwtSvc:  jwtSvc,
		authSvc: authService.NewService(jwtSvc),
	}

	pair, err := jwtSvc.GenerateTokenPair("user-a", "user-a", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if err := handler.revokeRefreshToken(context.Background(), pair.RefreshToken, "user-a"); err != nil {
		t.Fatalf("revokeRefreshToken() error = %v", err)
	}
	if len(store.added) != 1 {
		t.Fatalf("blacklist Add calls = %d, want 1", len(store.added))
	}
	accessClaims, err := jwtSvc.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	isBlacklisted, blacklistErr := jwtSvc.IsBlacklisted(context.Background(), accessClaims.ID)
	if blacklistErr != nil || !isBlacklisted {
		t.Fatalf("IsBlacklisted() = (%v, %v), want (true, nil)", isBlacklisted, blacklistErr)
	}
}

func TestRevokeAuthTokensDoesNotBlacklistAccessTokenWhenRefreshTokenMismatches(t *testing.T) {
	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	handler := &AuthHandler{
		jwtSvc:  jwtSvc,
		authSvc: authService.NewService(jwtSvc),
	}

	pairA, err := jwtSvc.GenerateTokenPair("user-a", "user-a", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}
	pairB, err := jwtSvc.GenerateTokenPair("user-b", "user-b", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set(middleware.AuthorizationHeader, middleware.BearerPrefix+pairA.AccessToken)
	c.Request = req

	err = handler.revokeAuthTokens(c, pairB.RefreshToken, "user-a")
	if err != errLogoutRefreshTokenUserMismatch {
		t.Fatalf("revokeAuthTokens() error = %v, want %v", err, errLogoutRefreshTokenUserMismatch)
	}
	if len(store.added) != 0 {
		t.Fatalf("blacklist Add calls = %d, want 0", len(store.added))
	}
}

func TestRevokeAuthTokensRejectsRefreshTokenFromDifferentSession(t *testing.T) {
	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	handler := &AuthHandler{
		jwtSvc:  jwtSvc,
		authSvc: authService.NewService(jwtSvc),
	}

	pairA, err := jwtSvc.GenerateTokenPair("user-a", "user-a", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}
	pairB, err := jwtSvc.GenerateTokenPair("user-a", "user-a", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set(middleware.AuthorizationHeader, middleware.BearerPrefix+pairA.AccessToken)
	c.Request = req

	err = handler.revokeAuthTokens(c, pairB.RefreshToken, "user-a")
	if err != errLogoutRefreshTokenSessionMismatch {
		t.Fatalf("revokeAuthTokens() error = %v, want %v", err, errLogoutRefreshTokenSessionMismatch)
	}
	if len(store.added) != 0 {
		t.Fatalf("blacklist Add calls = %d, want 0", len(store.added))
	}
}

func TestRevokeAuthTokensWithoutRefreshTokenBlacklistsSession(t *testing.T) {
	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	handler := &AuthHandler{
		jwtSvc:  jwtSvc,
		authSvc: authService.NewService(jwtSvc),
	}

	pair, err := jwtSvc.GenerateTokenPair("user-a", "user-a", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set(middleware.AuthorizationHeader, middleware.BearerPrefix+pair.AccessToken)
	c.Request = req

	if err := handler.revokeAuthTokens(c, "", "user-a"); err != nil {
		t.Fatalf("revokeAuthTokens() error = %v", err)
	}
	if len(store.added) != 1 {
		t.Fatalf("blacklist Add calls = %d, want 1", len(store.added))
	}
	if _, err := jwtSvc.ValidateRefreshToken(pair.RefreshToken); err != jwt.ErrInvalidToken {
		t.Fatalf("ValidateRefreshToken() after logout error = %v, want %v", err, jwt.ErrInvalidToken)
	}
}

func TestRevokeAuthTokensFailsWithoutPartialStateWhenSessionRevokeFails(t *testing.T) {
	store := &failingLogoutBlacklistRecorder{failOnCall: 1}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	handler := &AuthHandler{
		jwtSvc:  jwtSvc,
		authSvc: authService.NewService(jwtSvc),
	}

	pair, err := jwtSvc.GenerateTokenPair("user-a", "user-a", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set(middleware.AuthorizationHeader, middleware.BearerPrefix+pair.AccessToken)
	c.Request = req

	err = handler.revokeAuthTokens(c, pair.RefreshToken, "user-a")
	if err == nil || !strings.Contains(err.Error(), "撤销当前会话失败") {
		t.Fatalf("revokeAuthTokens() error = %v, want session revoke failure", err)
	}
	if len(store.added) != 0 {
		t.Fatalf("blacklist Add calls = %d, want 0", len(store.added))
	}
}

func TestRevokeAuthTokensRejectsLegacySessionRefreshLogout(t *testing.T) {
	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	handler := &AuthHandler{
		jwtSvc:  jwtSvc,
		authSvc: authService.NewService(jwtSvc),
	}

	accessToken, accessJTI, err := signLegacyAccessToken("logout-test", "logout-test", "user-a")
	if err != nil {
		t.Fatalf("signLegacyAccessToken() error = %v", err)
	}
	refreshToken, refreshJTI, err := signLegacyRefreshToken("logout-test", "logout-test", "user-a")
	if err != nil {
		t.Fatalf("signLegacyRefreshToken() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set(middleware.AuthorizationHeader, middleware.BearerPrefix+accessToken)
	c.Request = req

	err = handler.revokeAuthTokens(c, refreshToken, "user-a")
	if err != errLogoutLegacyRefreshUnsupported {
		t.Fatalf("revokeAuthTokens() error = %v, want %v", err, errLogoutLegacyRefreshUnsupported)
	}
	if isBlacklisted, blacklistErr := jwtSvc.IsBlacklisted(context.Background(), accessJTI); blacklistErr != nil || isBlacklisted {
		t.Fatalf("legacy access blacklist = (%v, %v), want (false, nil)", isBlacklisted, blacklistErr)
	}
	if isBlacklisted, blacklistErr := jwtSvc.IsBlacklisted(context.Background(), refreshJTI); blacklistErr != nil || isBlacklisted {
		t.Fatalf("legacy refresh blacklist = (%v, %v), want (false, nil)", isBlacklisted, blacklistErr)
	}
}

func TestRevokeAuthTokensSupportsLegacyAccessOnlyLogout(t *testing.T) {
	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	handler := &AuthHandler{
		jwtSvc:  jwtSvc,
		authSvc: authService.NewService(jwtSvc),
	}

	accessToken, accessJTI, err := signLegacyAccessToken("logout-test", "logout-test", "user-a")
	if err != nil {
		t.Fatalf("signLegacyAccessToken() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set(middleware.AuthorizationHeader, middleware.BearerPrefix+accessToken)
	c.Request = req

	if err := handler.revokeAuthTokens(c, "", "user-a"); err != nil {
		t.Fatalf("revokeAuthTokens() error = %v", err)
	}
	if isBlacklisted, blacklistErr := jwtSvc.IsBlacklisted(context.Background(), accessJTI); blacklistErr != nil || !isBlacklisted {
		t.Fatalf("legacy access blacklist = (%v, %v), want (true, nil)", isBlacklisted, blacklistErr)
	}
}

func TestCurrentTenantOrNilPrefersHeaderTenantClaim(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	tenantA := uuid.New()
	tenantB := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("X-Tenant-ID", tenantB.String())
	c.Request = req
	c.Set(middleware.TenantIDsKey, []string{tenantA.String(), tenantB.String()})
	c.Set(middleware.DefaultTenantIDKey, tenantA.String())

	if got := currentTenantOrNil(c); got != tenantB {
		t.Fatalf("currentTenantOrNil() = %v, want %v", got, tenantB)
	}
}

func TestCurrentTenantOrNilFallsBackToDefaultTenantClaim(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	tenantID := uuid.New()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	c.Set(middleware.DefaultTenantIDKey, tenantID.String())

	if got := currentTenantOrNil(c); got != tenantID {
		t.Fatalf("currentTenantOrNil() = %v, want %v", got, tenantID)
	}
}

func TestRefreshTokenReturnsInternalErrorWhenUserInfoReloadFails(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	insertUser(t, db, userID, "tenant-user", false)
	mustExecAuthSQL(t, db, `DROP TABLE permissions;`)

	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "handler-test",
	}, store)
	router := newAuthHandlerTestRouterWithJWTService(t, db, jwtSvc)
	pair, err := jwtSvc.GenerateTokenPair(userID.String(), "tenant-user", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	body := []byte(fmt.Sprintf(`{"refresh_token":%q}`, pair.RefreshToken))
	recorder := issueAuthRequest(t, router, http.MethodPost, "/api/v1/auth/refresh", "", nil, body)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if _, err := jwtSvc.ValidateRefreshToken(pair.RefreshToken); err != nil {
		t.Fatalf("ValidateRefreshToken() after failed refresh error = %v, want nil", err)
	}
	if len(store.added) != 0 {
		t.Fatalf("blacklist Add calls = %d, want 0", len(store.added))
	}
}

func signLegacyAccessToken(secret, issuer, userID string) (string, string, error) {
	jti := uuid.NewString()
	claims := jwt.Claims{
		RegisteredClaims: golangjwt.RegisteredClaims{
			ID:        jti,
			Subject:   userID,
			Issuer:    issuer,
			Audience:  golangjwt.ClaimStrings{"access"},
			IssuedAt:  golangjwt.NewNumericDate(time.Now()),
			ExpiresAt: golangjwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Username: "legacy-user",
	}
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	return signed, jti, err
}

func signLegacyRefreshToken(secret, issuer, userID string) (string, string, error) {
	jti := uuid.NewString()
	claims := golangjwt.RegisteredClaims{
		ID:        jti,
		Subject:   userID,
		Issuer:    issuer,
		Audience:  golangjwt.ClaimStrings{"refresh"},
		IssuedAt:  golangjwt.NewNumericDate(time.Now()),
		ExpiresAt: golangjwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	return signed, jti, err
}
