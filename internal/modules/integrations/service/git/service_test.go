package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRepoFilePathRejectsSymlinkEscape(t *testing.T) {
	repoRoot := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")

	if err := os.WriteFile(outsideFile, []byte("secret"), 0600); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(repoRoot, "escape.txt")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, err := resolveRepoFilePath(repoRoot, "escape.txt")
	if err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestResolveRepoFilePathAllowsRegularFile(t *testing.T) {
	repoRoot := t.TempDir()
	filePath := filepath.Join(repoRoot, "play.yml")

	if err := os.WriteFile(filePath, []byte("---\n"), 0600); err != nil {
		t.Fatalf("WriteFile(repo): %v", err)
	}

	resolved, err := resolveRepoFilePath(repoRoot, "play.yml")
	if err != nil {
		t.Fatalf("resolveRepoFilePath() error = %v", err)
	}
	if resolved != filePath {
		t.Fatalf("resolved = %q, want %q", resolved, filePath)
	}
}

func TestRepoFileExistsRejectsTraversal(t *testing.T) {
	parentDir := t.TempDir()
	repoRoot := filepath.Join(parentDir, "repo")
	outsideFile := filepath.Join(parentDir, "outside.yml")

	if err := os.Mkdir(repoRoot, 0755); err != nil {
		t.Fatalf("Mkdir(repo): %v", err)
	}
	if err := os.WriteFile(outsideFile, []byte("---\n"), 0600); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}

	if _, err := repoFileExists(repoRoot, "../outside.yml"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}
