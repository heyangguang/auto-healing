package main

import (
	"os"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/pkg/crypto"
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
