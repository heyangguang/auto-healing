package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/google/uuid"
)

func TestResolveInitialAdminPasswordUsesEnv(t *testing.T) {
	t.Setenv("INIT_ADMIN_PASSWORD", "from-env")
	password, err := resolveInitialAdminPassword()
	if err != nil {
		t.Fatalf("resolveInitialAdminPassword() error = %v", err)
	}
	if password != "from-env" {
		t.Fatalf("password = %q, want from-env", password)
	}
}

func TestResolveInitialAdminPasswordGeneratesRandomValue(t *testing.T) {
	_ = os.Unsetenv("INIT_ADMIN_PASSWORD")
	password, err := resolveInitialAdminPassword()
	if err != nil {
		t.Fatalf("resolveInitialAdminPassword() error = %v", err)
	}
	if strings.TrimSpace(password) == "" {
		t.Fatal("password should not be empty")
	}
	if _, err := crypto.HashPassword(password); err != nil {
		t.Fatalf("generated password should be hashable: %v", err)
	}
}

func TestBindPlatformAdminRoleWithFailsWhenRoleLookupFails(t *testing.T) {
	err := bindPlatformAdminRoleWith(
		func(context.Context, string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("not found")
		},
		func(context.Context, uuid.UUID, []uuid.UUID) error { return nil },
		context.Background(),
		uuid.New(),
	)
	if err == nil {
		t.Fatal("bindPlatformAdminRoleWith() error = nil, want lookup failure")
	}
}

func TestBindPlatformAdminRoleWithFailsWhenAssignFails(t *testing.T) {
	err := bindPlatformAdminRoleWith(
		func(context.Context, string) (uuid.UUID, error) {
			return uuid.New(), nil
		},
		func(context.Context, uuid.UUID, []uuid.UUID) error {
			return errors.New("assign failed")
		},
		context.Background(),
		uuid.New(),
	)
	if err == nil {
		t.Fatal("bindPlatformAdminRoleWith() error = nil, want assign failure")
	}
}
