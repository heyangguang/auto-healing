package jwt

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryBlacklist struct {
	items map[string]time.Time
}

type contextProbeBlacklist struct {
	lastValue string
}

type errorBlacklist struct{}

func (m *memoryBlacklist) Add(_ context.Context, jti string, exp time.Time) error {
	if m.items == nil {
		m.items = make(map[string]time.Time)
	}
	m.items[jti] = exp
	return nil
}

func (m *memoryBlacklist) Exists(_ context.Context, jti string) (bool, error) {
	exp, ok := m.items[jti]
	if !ok {
		return false, nil
	}
	return exp.After(time.Now()), nil
}

func (p *contextProbeBlacklist) Add(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (p *contextProbeBlacklist) Exists(ctx context.Context, _ string) (bool, error) {
	if value, _ := ctx.Value("probe").(string); value != "" {
		p.lastValue = value
	}
	return false, nil
}

func (errorBlacklist) Add(context.Context, string, time.Time) error {
	return nil
}

func (errorBlacklist) Exists(context.Context, string) (bool, error) {
	return false, errors.New("redis down")
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
	accessClaims, err := svc.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	isBlacklisted, blacklistErr := svc.IsBlacklisted(context.Background(), accessClaims.ID)
	if blacklistErr != nil || !isBlacklisted {
		t.Fatalf("IsBlacklisted() = (%v, %v), want (true, nil)", isBlacklisted, blacklistErr)
	}
}

func TestValidateRefreshTokenContextPassesCallerContextToBlacklist(t *testing.T) {
	store := &contextProbeBlacklist{}
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

	ctx := context.WithValue(context.Background(), "probe", "refresh-path")
	if _, err := svc.ValidateRefreshTokenContext(ctx, pair.RefreshToken); err != nil {
		t.Fatalf("ValidateRefreshTokenContext() error = %v", err)
	}
	if store.lastValue != "refresh-path" {
		t.Fatalf("blacklist lastValue = %q, want %q", store.lastValue, "refresh-path")
	}
}

func TestValidateRefreshTokenRejectsAccessToken(t *testing.T) {
	svc := NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "unit-test",
	}, &memoryBlacklist{})

	pair, err := svc.GenerateTokenPair("user-1", "tester", []string{"viewer"}, []string{"task:list"})
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if _, err := svc.ValidateRefreshToken(pair.AccessToken); err != ErrInvalidToken {
		t.Fatalf("ValidateRefreshToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestValidateTokenRejectsRefreshToken(t *testing.T) {
	svc := NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "unit-test",
	}, &memoryBlacklist{})

	pair, err := svc.GenerateTokenPair("user-1", "tester", []string{"viewer"}, []string{"task:list"})
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if _, err := svc.ValidateToken(pair.RefreshToken); err != ErrInvalidToken {
		t.Fatalf("ValidateToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestValidateTokenRejectsWrongIssuer(t *testing.T) {
	issuerA := NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "issuer-a",
	}, &memoryBlacklist{})
	issuerB := NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "issuer-b",
	}, &memoryBlacklist{})

	pair, err := issuerA.GenerateTokenPair("user-1", "tester", []string{"viewer"}, []string{"task:list"})
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if _, err := issuerB.ValidateToken(pair.AccessToken); err != ErrInvalidToken {
		t.Fatalf("ValidateToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestValidateRefreshTokenRejectsWrongIssuer(t *testing.T) {
	issuerA := NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "issuer-a",
	}, &memoryBlacklist{})
	issuerB := NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "issuer-b",
	}, &memoryBlacklist{})

	pair, err := issuerA.GenerateTokenPair("user-1", "tester", []string{"viewer"}, []string{"task:list"})
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if _, err := issuerB.ValidateRefreshToken(pair.RefreshToken); err != ErrInvalidToken {
		t.Fatalf("ValidateRefreshToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestNewServicePanicsWithoutBlacklistStore(t *testing.T) {
	defer func() {
		if r := recover(); r != ErrBlacklistStoreRequired {
			t.Fatalf("panic = %v, want %v", r, ErrBlacklistStoreRequired)
		}
	}()

	_ = NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "unit-test",
	}, nil)
}

func TestGenerateTokenPairUsesSharedSessionJTI(t *testing.T) {
	svc := NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "unit-test",
	}, &memoryBlacklist{})

	pair, err := svc.GenerateTokenPair("user-1", "tester", []string{"viewer"}, []string{"task:list"})
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	accessClaims, err := svc.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	refreshClaims, err := svc.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken() error = %v", err)
	}
	if accessClaims.ID != refreshClaims.ID {
		t.Fatalf("session jti mismatch: access=%q refresh=%q", accessClaims.ID, refreshClaims.ID)
	}
	if accessClaims.SessionExpiresAt != refreshClaims.ExpiresAt.Time.Unix() {
		t.Fatalf("session_expires_at = %d, want %d", accessClaims.SessionExpiresAt, refreshClaims.ExpiresAt.Time.Unix())
	}
}

func TestValidateRefreshTokenReturnsBlacklistLookupError(t *testing.T) {
	svc := NewService(Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "unit-test",
	}, errorBlacklist{})

	pair, err := svc.GenerateTokenPair("user-1", "tester", []string{"viewer"}, []string{"task:list"})
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if _, err := svc.ValidateRefreshToken(pair.RefreshToken); err != ErrBlacklistLookupFailed {
		t.Fatalf("ValidateRefreshToken() error = %v, want %v", err, ErrBlacklistLookupFailed)
	}
}
