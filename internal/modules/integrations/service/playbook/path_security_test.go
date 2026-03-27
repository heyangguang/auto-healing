package playbook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExistingRepoPathRejectsSymlinkEscape(t *testing.T) {
	repoRoot := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.yml")

	if err := os.WriteFile(outsideFile, []byte("---\n"), 0600); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(repoRoot, "escape.yml")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	if _, err := resolveExistingRepoPath(repoRoot, "escape.yml"); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestVariableScannerRejectsIncludedFileEscape(t *testing.T) {
	parentDir := t.TempDir()
	repoRoot := filepath.Join(parentDir, "repo")
	if err := os.Mkdir(repoRoot, 0755); err != nil {
		t.Fatalf("Mkdir(repo): %v", err)
	}

	rootPlaybook := filepath.Join(repoRoot, "site.yml")
	if err := os.WriteFile(rootPlaybook, []byte("- import_playbook: ../outside.yml\n"), 0600); err != nil {
		t.Fatalf("WriteFile(site): %v", err)
	}
	if err := os.WriteFile(filepath.Join(parentDir, "outside.yml"), []byte("---\n"), 0600); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}

	scanner := &VariableScanner{
		basePath:     repoRoot,
		scannedFiles: make(map[string]bool),
		variables:    make(map[string]*ScannedVariable),
	}
	if err := scanner.ScanFile(rootPlaybook); err == nil {
		t.Fatal("expected include escape to be rejected")
	}
}
