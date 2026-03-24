package jwt

import (
	"context"
	"testing"
	"time"
)

type memoryBlacklist struct {
	items map[string]time.Time
}

func (m *memoryBlacklist) Add(_ context.Context, jti string, exp time.Time) error {
	if m.items == nil {
		m.items = make(map[string]time.Time)
	}
	m.items[jti] = exp
	return nil
}

func (m *memoryBlacklist) Exists(_ context.Context, jti string) bool {
	exp, ok := m.items[jti]
	if !ok {
		return false
	}
	return exp.After(time.Now())
}

func TestValidateRefreshTokenRejectsBlacklistedToken(t *testing.T) {
	store := &memoryBlacklist{}
	svc := NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "unit-test",
	}, store)

	pair, err := svc.GenerateTokenPair("user-1", "tester", []string{"viewer"}, []string{"task:list"})
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := svc.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken() before blacklist error = %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("ValidateRefreshToken() subject = %q, want %q", claims.Subject, "user-1")
	}

	if err := svc.Blacklist(context.Background(), claims.ID, claims.ExpiresAt.Time); err != nil {
		t.Fatalf("Blacklist() error = %v", err)
	}

	if _, err := svc.ValidateRefreshToken(pair.RefreshToken); err != ErrInvalidToken {
		t.Fatalf("ValidateRefreshToken() after blacklist error = %v, want %v", err, ErrInvalidToken)
	}
}
