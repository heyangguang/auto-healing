package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/gin-gonic/gin"
	golangjwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

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
		authSvc: authService.NewServiceWithDeps(authService.ServiceDeps{JWTService: jwtSvc}),
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
		authSvc: authService.NewServiceWithDeps(authService.ServiceDeps{JWTService: jwtSvc}),
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
