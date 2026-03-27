package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/pkg/jwt"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	"github.com/google/uuid"
)

type refreshLookupErrorBlacklistStore struct {
	addCalls int
}

func (s *refreshLookupErrorBlacklistStore) Add(context.Context, string, time.Time) error {
	s.addCalls++
	return nil
}

func (*refreshLookupErrorBlacklistStore) Exists(context.Context, string) (bool, error) {
	return false, fmt.Errorf("redis down")
}

func TestRefreshTokenRouteReturnsInternalErrorWhenBlacklistLookupFails(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	store := &refreshLookupErrorBlacklistStore{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "handler-test",
	}, store)
	router := newAuthHandlerTestRouterWithJWTService(t, db, jwtSvc)
	pair, err := jwtSvc.GenerateTokenPair(uuid.NewString(), "tenant-user", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	body := []byte(fmt.Sprintf(`{"refresh_token":%q}`, pair.RefreshToken))
	recorder := issueAuthRequest(t, router, http.MethodPost, "/api/v1/auth/refresh", "", nil, body)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if store.addCalls != 0 {
		t.Fatalf("blacklist Add calls = %d, want 0", store.addCalls)
	}
}

func TestLogoutRouteRejectsLegacyAccessAndRefreshTokens(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	insertUser(t, db, userID, "legacy-user", false)

	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	router := newAuthHandlerTestRouterWithJWTService(t, db, jwtSvc)

	accessToken, accessJTI, err := signLegacyAccessToken("logout-test", "logout-test", userID.String())
	if err != nil {
		t.Fatalf("signLegacyAccessToken() error = %v", err)
	}
	refreshToken, refreshJTI, err := signLegacyRefreshToken("logout-test", "logout-test", userID.String())
	if err != nil {
		t.Fatalf("signLegacyRefreshToken() error = %v", err)
	}

	body := []byte(fmt.Sprintf(`{"refresh_token":%q}`, refreshToken))
	recorder := issueAuthRequest(t, router, http.MethodPost, "/api/v1/auth/logout", accessToken, nil, body)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if isBlacklisted, blacklistErr := jwtSvc.IsBlacklisted(context.Background(), accessJTI); blacklistErr != nil || isBlacklisted {
		t.Fatalf("legacy access blacklist = (%v, %v), want (false, nil)", isBlacklisted, blacklistErr)
	}
	if isBlacklisted, blacklistErr := jwtSvc.IsBlacklisted(context.Background(), refreshJTI); blacklistErr != nil || isBlacklisted {
		t.Fatalf("legacy refresh blacklist = (%v, %v), want (false, nil)", isBlacklisted, blacklistErr)
	}
}

func TestLogoutRouteSupportsLegacyAccessOnly(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	insertUser(t, db, userID, "legacy-user", false)

	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "logout-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "logout-test",
	}, store)
	router := newAuthHandlerTestRouterWithJWTService(t, db, jwtSvc)

	accessToken, accessJTI, err := signLegacyAccessToken("logout-test", "logout-test", userID.String())
	if err != nil {
		t.Fatalf("signLegacyAccessToken() error = %v", err)
	}

	recorder := issueAuthRequest(t, router, http.MethodPost, "/api/v1/auth/logout", accessToken, nil, nil)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if isBlacklisted, blacklistErr := jwtSvc.IsBlacklisted(context.Background(), accessJTI); blacklistErr != nil || !isBlacklisted {
		t.Fatalf("legacy access blacklist = (%v, %v), want (true, nil)", isBlacklisted, blacklistErr)
	}

	platformlifecycle.Cleanup()

	var audit struct {
		ID     string
		Action string
	}
	if err := db.Table("audit_logs").Select("id, action").Where("username = ?", "legacy-user").Take(&audit).Error; err != nil {
		t.Fatalf("load logout audit: %v", err)
	}
	if audit.ID == "" {
		t.Fatal("logout audit id = empty, want generated id")
	}
	if audit.Action != "logout" {
		t.Fatalf("logout audit action = %q, want %q", audit.Action, "logout")
	}
}
