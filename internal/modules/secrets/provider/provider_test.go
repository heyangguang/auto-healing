package provider

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
)

func TestNewFileProviderRejectsNonSSHAuthType(t *testing.T) {
	source := &secretsmodel.SecretsSource{
		Name:     "file-source",
		Type:     "file",
		AuthType: "password",
		Config: modeltypes.JSON{
			"key_path": "/etc/auto-healing/secrets/id_rsa",
		},
	}

	_, err := NewFileProvider(source)
	if !errors.Is(err, ErrProviderInvalidConfig) {
		t.Fatalf("expected ErrProviderInvalidConfig, got %v", err)
	}
}

func TestFileProviderRejectsSymlinkEscape(t *testing.T) {
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()
	originalPrefixes := allowedPathPrefixes
	allowedPathPrefixes = []string{allowedDir + string(os.PathSeparator)}
	t.Cleanup(func() {
		allowedPathPrefixes = originalPrefixes
	})

	targetPath := filepath.Join(outsideDir, "id_rsa")
	if err := os.WriteFile(targetPath, []byte("private-key"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	symlinkPath := filepath.Join(allowedDir, "link")
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	source := &secretsmodel.SecretsSource{
		Name:     "file-source",
		Type:     "file",
		AuthType: "ssh_key",
		Config: modeltypes.JSON{
			"key_path": symlinkPath,
		},
	}

	_, err := NewFileProvider(source)
	if !errors.Is(err, ErrProviderInvalidConfig) {
		t.Fatalf("expected ErrProviderInvalidConfig, got %v", err)
	}
}

func TestFileProviderRejectsDirectoryPathOnCreate(t *testing.T) {
	allowedDir := t.TempDir()
	originalPrefixes := allowedPathPrefixes
	allowedPathPrefixes = []string{allowedDir + string(os.PathSeparator)}
	t.Cleanup(func() {
		allowedPathPrefixes = originalPrefixes
	})

	source := &secretsmodel.SecretsSource{
		Name:     "file-source",
		Type:     "file",
		AuthType: "ssh_key",
		Config: modeltypes.JSON{
			"key_path": allowedDir,
		},
	}

	_, err := NewFileProvider(source)
	if !errors.Is(err, ErrProviderInvalidConfig) {
		t.Fatalf("expected ErrProviderInvalidConfig, got %v", err)
	}
}
