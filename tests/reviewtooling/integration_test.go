package reviewtooling

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupGeneratesReviewSessionArtifacts(t *testing.T) {
	sessionDir := setupSession(t, t.TempDir(), "integration-session")

	mustExist(t, filepath.Join(sessionDir, "modules.csv"))
	mustExist(t, filepath.Join(sessionDir, "module_csv_validator.awk"))
	mustExist(t, filepath.Join(sessionDir, "review_status.csv"))
	mustExist(t, filepath.Join(sessionDir, "repair_plan.csv"))
	mustExist(t, filepath.Join(sessionDir, "create_worktrees.sh"))
	mustExist(t, filepath.Join(sessionDir, "prompts", "auth_middleware.md"))
	mustExist(t, filepath.Join(sessionDir, "findings", "auth_middleware.md"))
	mustExist(t, filepath.Join(sessionDir, "README.md"))
}

func TestCreateWorktreesRejectsUnknownModuleBeforeMutation(t *testing.T) {
	sessionDir := setupSession(t, t.TempDir(), "integration-unknown-module")
	helper := filepath.Join(sessionDir, "create_worktrees.sh")
	fakeBin := filepath.Join(t.TempDir(), "bin")
	logPath := filepath.Join(t.TempDir(), "git.log")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("find git: %v", err)
	}

	mustWriteFile(t, filepath.Join(fakeBin, "git"), strings.Join([]string{
		"#!/bin/bash",
		"printf '%s\\n' \"$*\" >> " + logPath,
		"if [[ \"${1:-}\" == \"worktree\" && \"${2:-}\" == \"add\" ]]; then",
		"  printf 'unexpected worktree add\\n' >> " + logPath,
		"  exit 99",
		"fi",
		"exec " + realGit + " \"$@\"",
	}, "\n")+"\n", 0o755)
	mustWriteFile(t, logPath, "", 0o644)

	result := runCommand(
		t,
		repoRoot(t),
		map[string]string{"PATH": fakeBin + ":" + os.Getenv("PATH")},
		"bash",
		helper,
		"auth_middleware",
		"no_such_module",
	)
	if result.err == nil {
		t.Fatal("expected helper to reject unknown module")
	}
	if !strings.Contains(result.stderr, "unknown module_id: no_such_module") {
		t.Fatalf("expected unknown module error, got stderr:\n%s", result.stderr)
	}
	if strings.Contains(mustReadFile(t, logPath), "unexpected worktree add") {
		t.Fatalf("helper should not invoke worktree add before rejecting unknown module:\n%s", mustReadFile(t, logPath))
	}
}

func TestSetupFailsWithoutPythonBeforeCreatingSession(t *testing.T) {
	root := repoRoot(t)
	wrapperRoot := t.TempDir()
	fakeBin := filepath.Join(wrapperRoot, "bin")
	mustWriteFile(t, filepath.Join(fakeBin, "git"), "#!/bin/bash\nexec /usr/bin/git \"$@\"\n", 0o755)
	mustWriteFile(t, filepath.Join(fakeBin, "awk"), "#!/bin/bash\nexec /usr/bin/awk \"$@\"\n", 0o755)
	mustWriteFile(t, filepath.Join(fakeBin, "dirname"), "#!/bin/bash\nexec /usr/bin/dirname \"$@\"\n", 0o755)

	reviewRoot := filepath.Join(wrapperRoot, "review-root")
	result := runCommand(
		t,
		root,
		map[string]string{
			"PATH":        fakeBin,
			"REVIEW_ROOT": reviewRoot,
		},
		"/bin/bash",
		filepath.Join(root, "scripts/review/setup_parallel_review.sh"),
		"no-python-check",
	)
	if result.err == nil {
		t.Fatal("expected setup to fail without python")
	}
	if !strings.Contains(result.stderr, "required command not found: python3 or python") {
		t.Fatalf("expected missing python error, got stderr:\n%s", result.stderr)
	}
	if _, err := os.Stat(filepath.Join(reviewRoot, "no-python-check")); !os.IsNotExist(err) {
		t.Fatalf("expected no half-generated session, got stat err=%v", err)
	}
}
