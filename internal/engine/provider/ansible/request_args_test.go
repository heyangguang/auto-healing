package ansible

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveInventoryFilePrefersWorkDirRelativePath(t *testing.T) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	cwd := t.TempDir()
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir(cwd) error = %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})

	workDir := t.TempDir()
	if err := os.WriteFile("inventory.ini", []byte("[all]\ncwd-host\n"), 0600); err != nil {
		t.Fatalf("write cwd inventory: %v", err)
	}
	workInventory := filepath.Join(workDir, "inventory.ini")
	if err := os.WriteFile(workInventory, []byte("[all]\nworkdir-host\n"), 0600); err != nil {
		t.Fatalf("write workdir inventory: %v", err)
	}

	resolved, err := resolveInventoryFile(workDir, "inventory.ini")
	if err != nil {
		t.Fatalf("resolveInventoryFile() error = %v", err)
	}
	if resolved != workInventory {
		t.Fatalf("resolved inventory = %q, want %q", resolved, workInventory)
	}
}

func TestResolveInventoryFileDoesNotFallBackToCurrentWorkingDirectory(t *testing.T) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	cwd := t.TempDir()
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir(cwd) error = %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})

	if err := os.WriteFile("inventory.ini", []byte("[all]\ncwd-host\n"), 0600); err != nil {
		t.Fatalf("write cwd inventory: %v", err)
	}

	resolved, err := resolveInventoryFile(t.TempDir(), "inventory.ini")
	if err != nil {
		t.Fatalf("resolveInventoryFile() error = %v", err)
	}
	if resolved != "" {
		t.Fatalf("resolved inventory = %q, want empty path", resolved)
	}
}

func TestResolveInventoryFileRejectsRelativePathOutsideWorkDir(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	if err := os.MkdirAll(workDir, 0700); err != nil {
		t.Fatalf("MkdirAll(workDir) error = %v", err)
	}
	outside := filepath.Join(root, "inventory.ini")
	if err := os.WriteFile(outside, []byte("[all]\noutside-host\n"), 0600); err != nil {
		t.Fatalf("write outside inventory: %v", err)
	}

	_, err := resolveInventoryFile(workDir, "../inventory.ini")
	if err == nil || !strings.Contains(err.Error(), "inventory 文件必须位于工作目录内") {
		t.Fatalf("expected workdir boundary error, got %v", err)
	}
}
