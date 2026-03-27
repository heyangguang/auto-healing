package httpapi

import (
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNewSSHClientConfigFailsWithoutKnownHosts(t *testing.T) {
	t.Setenv("AUTO_HEALING_KNOWN_HOSTS", filepath.Join(t.TempDir(), "missing_known_hosts"))

	if _, err := newSSHClientConfig("root", ssh.Password("secret")); err == nil {
		t.Fatal("expected missing known_hosts to return an error")
	}
}

func TestTestConnectionByAuthTypeRejectsUnknownType(t *testing.T) {
	if err := testConnectionByAuthType("127.0.0.1", "root", "secret", "", "unknown"); err == nil {
		t.Fatal("expected unsupported auth_type to return an error")
	}
}
