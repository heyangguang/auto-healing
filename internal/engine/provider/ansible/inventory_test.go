package ansible

import (
	"strings"
	"testing"
)

func TestGenerateInventoryWithAuthQuotesCredentialValues(t *testing.T) {
	content := GenerateInventoryWithAuth([]HostCredential{{
		Host:     "host-a",
		AuthType: "password",
		Username: "user name",
		Password: "pa ss#word\nline2",
	}}, "targets")

	lines := strings.Split(content, "\n")
	if !strings.Contains(lines[1], `ansible_user="user name"`) {
		t.Fatalf("host line missing quoted username: %q", lines[1])
	}
	if !strings.Contains(lines[1], `ansible_ssh_pass="pa ss#word\nline2"`) {
		t.Fatalf("host line missing quoted password: %q", lines[1])
	}
}

func TestGenerateInventoryQuotesGroupVars(t *testing.T) {
	content := GenerateInventory("host-a", "targets", map[string]string{
		"ansible_user": "user name",
	})
	if !strings.Contains(content, "ansible_user=\"user name\"") {
		t.Fatalf("inventory missing quoted group var: %q", content)
	}
}

func TestWriteInventoryFileRejectsInvalidGeneratedHost(t *testing.T) {
	_, err := WriteInventoryFile(t.TempDir(), GenerateInventory("host-a,bad host", "targets", nil))
	if err == nil || !strings.Contains(err.Error(), "inventory 主机格式非法") {
		t.Fatalf("expected invalid host error, got %v", err)
	}
}
