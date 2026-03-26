package ansible

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestWorkspaceManagerPrepareWorkspaceRejectsExternalSymlink(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		t.Fatalf("MkdirAll(repoDir) error = %v", err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink("../outside.txt", filepath.Join(repoDir, "leak.txt")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	manager := NewWorkspaceManager()
	_, _, err := manager.PrepareWorkspace(uuid.New(), repoDir)
	if err == nil || !strings.Contains(err.Error(), "仓库外") {
		t.Fatalf("expected external symlink rejection, got %v", err)
	}
}

func TestWorkspaceManagerPrepareWorkspacePreservesInternalRelativeSymlink(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := filepath.Join(repoDir, "sub")
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		t.Fatalf("MkdirAll(targetDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "data.txt"), []byte("ok"), 0600); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if err := os.Symlink("sub/data.txt", filepath.Join(repoDir, "link.txt")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	manager := NewWorkspaceManager()
	workDir, cleanup, err := manager.PrepareWorkspace(uuid.New(), repoDir)
	if err != nil {
		t.Fatalf("PrepareWorkspace() error = %v", err)
	}
	defer cleanup()

	info, err := os.Lstat(filepath.Join(workDir, "link.txt"))
	if err != nil {
		t.Fatalf("Lstat(link) error = %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected copied link to remain a symlink, mode=%v", info.Mode())
	}
	target, err := os.Readlink(filepath.Join(workDir, "link.txt"))
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if target != "sub/data.txt" {
		t.Fatalf("copied symlink target = %q, want %q", target, "sub/data.txt")
	}
}
