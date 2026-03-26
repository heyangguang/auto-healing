package config

import (
	"os"
	"path/filepath"
	"strings"
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
	unsetEnvForTest(t, "APP_ENV", "JWT_SECRET", "AUTO_HEALING_CONFIG_FILE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.JWT.Secret == "" || cfg.JWT.Secret == "your-super-secret-key-change-in-production" {
		t.Fatalf("JWT secret was not randomized: %q", cfg.JWT.Secret)
	}
	if cfg.App.Env != "development" {
		t.Fatalf("App.Env = %q, want development", cfg.App.Env)
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
	unsetEnvForTest(t, "AUTO_HEALING_CONFIG_FILE")

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

func TestLoadRejectsPlaceholderJWTSecretInProduction(t *testing.T) {
	tmpDir := t.TempDir()
	restoreWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() { _ = os.Chdir(restoreWD) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	unsetEnvForTest(t, "APP_ENV", "JWT_SECRET", "AUTO_HEALING_CONFIG_FILE")

	cfgDir := filepath.Join(tmpDir, "configs")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := []byte("app:\n  env: production\njwt:\n  secret: your-super-secret-key-change-in-production\n")
	configPath := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Fatal("Load() error = nil, want production validation error")
	}
	if !strings.Contains(err.Error(), "jwt.secret") {
		t.Fatalf("error = %q, want jwt.secret hint", err)
	}
	if !strings.Contains(err.Error(), configPath) {
		t.Fatalf("error = %q, want config path %q", err, configPath)
	}
}

func TestLoadRequiredRejectsMissingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	restoreWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() { _ = os.Chdir(restoreWD) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	unsetEnvForTest(t, "AUTO_HEALING_CONFIG_FILE")

	_, err = LoadRequired()
	if err == nil {
		t.Fatal("LoadRequired() error = nil, want missing config error")
	}
	if !strings.Contains(err.Error(), "未找到配置文件") {
		t.Fatalf("error = %q, want missing config hint", err)
	}
}

func TestLoadRequiredUsesExplicitConfigFileEnv(t *testing.T) {
	unsetEnvForTest(t, "APP_ENV", "JWT_SECRET", "AUTO_HEALING_CONFIG_FILE")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom.yaml")
	content := []byte("app:\n  env: production\njwt:\n  secret: explicit-secret\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("AUTO_HEALING_CONFIG_FILE", configPath)

	cfg, err := LoadRequired()
	if err != nil {
		t.Fatalf("LoadRequired() error = %v", err)
	}
	if cfg.JWT.Secret != "explicit-secret" {
		t.Fatalf("JWT secret = %q, want explicit-secret", cfg.JWT.Secret)
	}
	if cfg.App.Env != "production" {
		t.Fatalf("App.Env = %q, want production", cfg.App.Env)
	}
}

func unsetEnvForTest(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		value, exists := os.LookupEnv(key)
		if exists {
			t.Cleanup(func() {
				_ = os.Setenv(key, value)
			})
		} else {
			t.Cleanup(func() {
				_ = os.Unsetenv(key)
			})
		}
		_ = os.Unsetenv(key)
	}
}
