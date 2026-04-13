package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/google/uuid"
)

func TestResolveTargetAdminUsernameUsesDefault(t *testing.T) {
	_ = os.Unsetenv("RESET_ADMIN_USERNAME")
	if username := resolveTargetAdminUsername(); username != "admin" {
		t.Fatalf("username = %q, want admin", username)
	}
}

func TestResolveTargetAdminUsernameUsesEnv(t *testing.T) {
	t.Setenv("RESET_ADMIN_USERNAME", "ops-admin")
	if username := resolveTargetAdminUsername(); username != "ops-admin" {
		t.Fatalf("username = %q, want ops-admin", username)
	}
}

func TestResolveResetAdminPasswordUsesEnv(t *testing.T) {
	t.Setenv("RESET_ADMIN_PASSWORD", "from-env")
	password, err := resolveResetAdminPassword()
	if err != nil {
		t.Fatalf("resolveResetAdminPassword() error = %v", err)
	}
	if password != "from-env" {
		t.Fatalf("password = %q, want from-env", password)
	}
}

func TestResolveResetAdminPasswordGeneratesRandomValue(t *testing.T) {
	_ = os.Unsetenv("RESET_ADMIN_PASSWORD")
	password, err := resolveResetAdminPassword()
	if err != nil {
		t.Fatalf("resolveResetAdminPassword() error = %v", err)
	}
	if strings.TrimSpace(password) == "" {
		t.Fatal("password should not be empty")
	}
	if _, err := crypto.HashPassword(password); err != nil {
		t.Fatalf("generated password should be hashable: %v", err)
	}
}

func TestResetPlatformAdminPasswordWithFailsForNonPlatformAdmin(t *testing.T) {
	_, err := resetPlatformAdminPasswordWith(
		func(context.Context, string) (*accessmodel.User, error) {
			return &accessmodel.User{ID: uuid.New(), Username: "admin", IsPlatformAdmin: false}, nil
		},
		func(context.Context, uuid.UUID, string) error { return nil },
		crypto.HashPassword,
		context.Background(),
		"admin",
		"new-password",
	)
	if !errors.Is(err, ErrTargetNotPlatformAdmin) {
		t.Fatalf("error = %v, want ErrTargetNotPlatformAdmin", err)
	}
}

func TestResetPlatformAdminPasswordWithUpdatesPasswordHash(t *testing.T) {
	var updatedUserID uuid.UUID
	var updatedHash string
	userID := uuid.New()

	user, err := resetPlatformAdminPasswordWith(
		func(context.Context, string) (*accessmodel.User, error) {
			return &accessmodel.User{ID: userID, Username: "admin", IsPlatformAdmin: true}, nil
		},
		func(_ context.Context, id uuid.UUID, passwordHash string) error {
			updatedUserID = id
			updatedHash = passwordHash
			return nil
		},
		crypto.HashPassword,
		context.Background(),
		"admin",
		"new-password",
	)
	if err != nil {
		t.Fatalf("resetPlatformAdminPasswordWith() error = %v", err)
	}
	if user.ID != userID {
		t.Fatalf("user.ID = %s, want %s", user.ID, userID)
	}
	if updatedUserID != userID {
		t.Fatalf("updatedUserID = %s, want %s", updatedUserID, userID)
	}
	if !crypto.CheckPassword("new-password", updatedHash) {
		t.Fatal("updated hash does not match password")
	}
}
