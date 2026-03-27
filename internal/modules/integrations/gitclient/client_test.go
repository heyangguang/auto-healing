package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/modules/integrations/model"
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
	if !strings.Contains(cmd, "UserKnownHostsFile='/tmp/custom-known-hosts'") {
		t.Fatalf("command missing custom known_hosts: %s", cmd)
	}
}

func TestBuildGitSSHCommandQuotesPaths(t *testing.T) {
	t.Setenv("AUTO_HEALING_KNOWN_HOSTS", "/tmp/known hosts")
	cmd, err := buildGitSSHCommand("/tmp/private key")
	if err != nil {
		t.Fatalf("buildGitSSHCommand() error = %v", err)
	}
	if !strings.Contains(cmd, "-i '/tmp/private key'") {
		t.Fatalf("command missing quoted key path: %s", cmd)
	}
	if !strings.Contains(cmd, "UserKnownHostsFile='/tmp/known hosts'") {
		t.Fatalf("command missing quoted known_hosts path: %s", cmd)
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

func TestGetAuthenticatedURLRejectsMissingToken(t *testing.T) {
	client := NewClient(&model.GitRepository{
		URL:      "https://github.com/company/private.git",
		AuthType: "token",
	}, t.TempDir())

	_, _, err := client.getAuthenticatedURL()
	if err == nil {
		t.Fatal("getAuthenticatedURL() error = nil, want missing token error")
	}
	if !strings.Contains(err.Error(), "缺少 token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAuthenticatedURLInjectsTokenIntoHTTPURL(t *testing.T) {
	client := NewClient(&model.GitRepository{
		URL:      "http://github.com/company/private.git",
		AuthType: "token",
		AuthConfig: map[string]any{
			"token": "secret",
		},
	}, t.TempDir())

	url, _, err := client.getAuthenticatedURL()
	if err != nil {
		t.Fatalf("getAuthenticatedURL() error = %v", err)
	}
	if url != "http://secret@github.com/company/private.git" {
		t.Fatalf("url = %q", url)
	}
}

func TestGetAuthenticatedURLRejectsPasswordAuthForUnsupportedScheme(t *testing.T) {
	client := NewClient(&model.GitRepository{
		URL:      "git@github.com:company/private.git",
		AuthType: "password",
		AuthConfig: map[string]any{
			"username": "user",
			"password": "pass",
		},
	}, t.TempDir())

	_, _, err := client.getAuthenticatedURL()
	if err == nil {
		t.Fatal("getAuthenticatedURL() error = nil, want scheme mismatch error")
	}
	if !strings.Contains(err.Error(), "不支持当前仓库地址协议") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAuthenticatedURLRejectsInvalidAuthConfig(t *testing.T) {
	client := NewClient(&model.GitRepository{
		URL:      "https://github.com/company/private.git",
		AuthType: "token",
		AuthConfig: map[string]any{
			"token": make(chan int),
		},
	}, t.TempDir())

	_, _, err := client.getAuthenticatedURL()
	if err == nil {
		t.Fatal("getAuthenticatedURL() error = nil, want invalid auth config error")
	}
	if !strings.Contains(err.Error(), "认证配置") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteSSHPrivateKeyFileFailsOnClosedFile(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "ssh-key-*")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	err = writeSSHPrivateKeyFile(tmpFile, "private-key")
	if err == nil {
		t.Fatal("writeSSHPrivateKeyFile() error = nil, want closed file error")
	}
}
