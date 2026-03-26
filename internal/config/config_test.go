package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGeneratesJWTSecretWhenUnset(t *testing.T) {
	tmpDir := t.TempDir()
	restoreWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() { _ = os.Chdir(restoreWD) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Setenv("APP_ENV", "development")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.JWT.Secret == "" || cfg.JWT.Secret == "your-super-secret-key-change-in-production" {
		t.Fatalf("JWT secret was not randomized: %q", cfg.JWT.Secret)
	}
}

func TestLoadUsesConfiguredJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()
	restoreWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() { _ = os.Chdir(restoreWD) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	cfgDir := filepath.Join(tmpDir, "configs")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := []byte("jwt:\n  secret: configured-secret\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.JWT.Secret != "configured-secret" {
		t.Fatalf("JWT secret = %q, want configured-secret", cfg.JWT.Secret)
	}
}
