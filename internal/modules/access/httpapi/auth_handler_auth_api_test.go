package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/google/uuid"
)

type handlerRouteErrorBlacklistStore struct{}

func (handlerRouteErrorBlacklistStore) Add(context.Context, string, time.Time) error { return nil }

func (handlerRouteErrorBlacklistStore) Exists(context.Context, string) (bool, error) {
	return false, errors.New("redis down")
}

func TestSetupAuthRoutesRefreshReturnsInternalErrorWhenBlacklistLookupFails(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "handler-refresh-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "handler-test",
	}, handlerRouteErrorBlacklistStore{})
	router := newAuthHandlerTestRouterWithJWTService(t, db, jwtSvc)
	pair, err := jwtSvc.GenerateTokenPair(uuid.NewString(), "tenant-user", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	body := []byte(`{"refresh_token":"` + pair.RefreshToken + `"}`)
	recorder := issueAuthRequest(t, router, http.MethodPost, "/api/v1/auth/refresh", "", nil, body)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
}

func TestSetupAuthRoutesLogoutRevokesCurrentSession(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	insertUser(t, db, userID, "tenant-user", false)
	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "handler-logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "handler-test",
	}, store)
	router := newAuthHandlerTestRouterWithJWTService(t, db, jwtSvc)
	pair, err := jwtSvc.GenerateTokenPair(userID.String(), "tenant-user", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	body := []byte(`{"refresh_token":"` + pair.RefreshToken + `"}`)
	recorder := issueAuthRequest(t, router, http.MethodPost, "/api/v1/auth/logout", pair.AccessToken, nil, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	claims, err := jwtSvc.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	isBlacklisted, blacklistErr := jwtSvc.IsBlacklisted(context.Background(), claims.ID)
	if blacklistErr != nil || !isBlacklisted {
		t.Fatalf("IsBlacklisted() = (%v, %v), want (true, nil)", isBlacklisted, blacklistErr)
	}
	if _, err := jwtSvc.ValidateRefreshToken(pair.RefreshToken); err != jwt.ErrInvalidToken {
		t.Fatalf("ValidateRefreshToken() error = %v, want %v", err, jwt.ErrInvalidToken)
	}
}

func TestSetupAuthRoutesLogoutRejectsLegacySessionRefreshPair(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	insertUser(t, db, userID, "tenant-user", false)
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "handler-logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "handler-test",
	}, &logoutBlacklistRecorder{})
	router := newAuthHandlerTestRouterWithJWTService(t, db, jwtSvc)
	accessToken, _, err := signLegacyAccessToken("handler-logout-test", "handler-test", userID.String())
	if err != nil {
		t.Fatalf("signLegacyAccessToken() error = %v", err)
	}
	refreshToken, _, err := signLegacyRefreshToken("handler-logout-test", "handler-test", userID.String())
	if err != nil {
		t.Fatalf("signLegacyRefreshToken() error = %v", err)
	}

	body := []byte(`{"refresh_token":"` + refreshToken + `"}`)
	recorder := issueAuthRequest(t, router, http.MethodPost, "/api/v1/auth/logout", accessToken, nil, body)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), errLogoutLegacyRefreshUnsupported.Error()) {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), errLogoutLegacyRefreshUnsupported.Error())
	}
}
