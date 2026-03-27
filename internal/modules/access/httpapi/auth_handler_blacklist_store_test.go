package httpapi

import (
	"context"
	"testing"
	"time"
)

func TestAuthTokenBlacklistStoreFailsClosedWithoutRedisClient(t *testing.T) {
	store := newAuthTokenBlacklistStore()

	isBlacklisted, err := store.Exists(context.Background(), "session-jti")
	if err == nil {
		t.Fatal("Exists() error = nil, want unavailable store error")
	}
	if !isBlacklisted {
		t.Fatal("Exists() = false, want true when redis client is unavailable")
	}
	if err := store.Add(context.Background(), "session-jti", time.Now().Add(time.Minute)); err == nil {
		t.Fatal("Add() error = nil, want unavailable store error")
	}
}
