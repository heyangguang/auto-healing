package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildGitSSHCommandUsesKnownHostsWhenConfigured(t *testing.T) {
	t.Setenv("AUTO_HEALING_KNOWN_HOSTS", "/tmp/custom-known-hosts")
	cmd, err := buildGitSSHCommand("/tmp/key")
	if err != nil {
		t.Fatalf("buildGitSSHCommand() error = %v", err)
	}
	if !strings.Contains(cmd, "StrictHostKeyChecking=yes") {
		t.Fatalf("command missing strict host key checking: %s", cmd)
	}
	if !strings.Contains(cmd, "UserKnownHostsFile=/tmp/custom-known-hosts") {
		t.Fatalf("command missing custom known_hosts: %s", cmd)
	}
}

func TestBuildGitSSHCommandFailsWithoutKnownHosts(t *testing.T) {
	t.Setenv("AUTO_HEALING_KNOWN_HOSTS", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	_, err := buildGitSSHCommand("/tmp/key")
	if err == nil {
		t.Fatal("expected missing known_hosts error")
	}
	if !strings.Contains(err.Error(), "AUTO_HEALING_KNOWN_HOSTS") {
		t.Fatalf("unexpected error: %v", err)
	}
	var knownHostsErr *KnownHostsRequiredError
	if !errors.As(err, &knownHostsErr) {
		t.Fatalf("expected KnownHostsRequiredError, got %T", err)
	}
	if knownHostsErr.ErrorCode() != ErrorCodeKnownHostsRequired {
		t.Fatalf("unexpected error code: %s", knownHostsErr.ErrorCode())
	}
}

func TestGitKnownHostsPathUsesDefaultHomeLocation(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("AUTO_HEALING_KNOWN_HOSTS", "")
	t.Setenv("HOME", tmpHome)

	knownHostsPath := filepath.Join(tmpHome, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(knownHostsPath, []byte("host key"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if got := gitKnownHostsPath(); got != knownHostsPath {
		t.Fatalf("gitKnownHostsPath() = %q, want %q", got, knownHostsPath)
	}
}

func TestRedactCredentialsMasksAuthenticatedURLs(t *testing.T) {
	raw := "fatal: could not read from https://ghp_secret-token@github.com/company/private.git"
	masked := redactCredentials(raw)

	if strings.Contains(masked, "ghp_secret-token") {
		t.Fatalf("credentials were not redacted: %s", masked)
	}
	if !strings.Contains(masked, "https://***@github.com/company/private.git") {
		t.Fatalf("masked output unexpected: %s", masked)
	}
}
