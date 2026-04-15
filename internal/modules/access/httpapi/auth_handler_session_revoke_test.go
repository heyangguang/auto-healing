package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/gin-gonic/gin"
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
		authSvc: authService.NewServiceWithDeps(authService.ServiceDeps{JWTService: jwtSvc}),
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
		authSvc: authService.NewServiceWithDeps(authService.ServiceDeps{JWTService: jwtSvc}),
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
	isRevoked, blacklistErr := jwtSvc.IsTokenRevoked(context.Background(), accessClaims.ID, accessClaims.SessionID)
	if blacklistErr != nil || isRevoked {
		t.Fatalf("IsTokenRevoked() = (%v, %v), want (false, nil)", isRevoked, blacklistErr)
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
		authSvc: authService.NewServiceWithDeps(authService.ServiceDeps{JWTService: jwtSvc}),
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
		authSvc: authService.NewServiceWithDeps(authService.ServiceDeps{JWTService: jwtSvc}),
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
		authSvc: authService.NewServiceWithDeps(authService.ServiceDeps{JWTService: jwtSvc}),
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
	accessClaims, err := jwtSvc.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	isRevoked, blacklistErr := jwtSvc.IsTokenRevoked(context.Background(), accessClaims.ID, accessClaims.SessionID)
	if blacklistErr != nil || !isRevoked {
		t.Fatalf("IsTokenRevoked() = (%v, %v), want (true, nil)", isRevoked, blacklistErr)
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
		authSvc: authService.NewServiceWithDeps(authService.ServiceDeps{JWTService: jwtSvc}),
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
