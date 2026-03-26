package handler

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/pkg/jwt"
	authService "github.com/company/auto-healing/internal/service/auth"
)

type logoutBlacklistRecorder struct {
	added []string
}

func (r *logoutBlacklistRecorder) Add(_ context.Context, jti string, _ time.Time) error {
	r.added = append(r.added, jti)
	return nil
}

func (r *logoutBlacklistRecorder) Exists(context.Context, string) bool {
	return false
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
}
