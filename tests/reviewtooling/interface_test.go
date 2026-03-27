package reviewtooling

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupHelpContract(t *testing.T) {
	root := repoRoot(t)
	result := runCommand(t, root, nil, "bash", filepath.Join(root, "scripts/review/setup_parallel_review.sh"), "--help")
	if result.err != nil {
		t.Fatalf("setup --help failed: %v\nstderr:\n%s", result.err, result.stderr)
	}
	for _, needle := range []string{"Usage:", "REVIEW_ROOT", "REVIEW_MODULES", "Required commands:"} {
		if !strings.Contains(result.stdout, needle) {
			t.Fatalf("expected setup --help to contain %q, got:\n%s", needle, result.stdout)
		}
	}
}

func TestGeneratedArtifactsMatchDocumentedContracts(t *testing.T) {
	sessionDir := setupSession(t, t.TempDir(), "interface-contract")
	statusCSV := mustReadFile(t, filepath.Join(sessionDir, "review_status.csv"))
	planCSV := mustReadFile(t, filepath.Join(sessionDir, "repair_plan.csv"))
	prompt := mustReadFile(t, filepath.Join(sessionDir, "prompts", "auth_middleware.md"))
	readme := mustReadFile(t, filepath.Join(sessionDir, "README.md"))

	if !strings.HasPrefix(statusCSV, "module_id,label,module_kind,module_note,status,branch_name,worktree_dir,owner,findings_file,notes\n") {
		t.Fatalf("unexpected review_status.csv header:\n%s", statusCSV)
	}
	if !strings.HasPrefix(planCSV, "module_id,label,module_kind,module_note,branch_name,worktree_dir,paths,shared_touchpoints,focus\n") {
		t.Fatalf("unexpected repair_plan.csv header:\n%s", planCSV)
	}
	if !strings.Contains(prompt, filepath.Join(sessionDir, "findings", "auth_middleware.md")) {
		t.Fatalf("prompt should contain absolute findings path:\n%s", prompt)
	}
	if !strings.Contains(readme, "Module CSV validator: `module_csv_validator.awk`") {
		t.Fatalf("README should mention module_csv_validator.awk:\n%s", readme)
	}
}

func TestCreateWorktreesHelpAndListContracts(t *testing.T) {
	sessionDir := setupSession(t, t.TempDir(), "interface-helper")
	helper := filepath.Join(sessionDir, "create_worktrees.sh")

	help := runCommand(t, sessionDir, nil, "bash", helper, "--help")
	if help.err != nil {
		t.Fatalf("helper --help failed: %v\nstderr:\n%s", help.err, help.stderr)
	}
	if !strings.Contains(help.stdout, "REVIEW_BASE_BRANCH overrides the session's default base ref") {
		t.Fatalf("unexpected helper help output:\n%s", help.stdout)
	}

	list := runCommand(t, sessionDir, map[string]string{"REVIEW_BASE_BRANCH": "definitely-not-a-branch"}, "bash", helper, "--list")
	if list.err != nil {
		t.Fatalf("helper --list should not depend on a valid base ref: %v\nstderr:\n%s", list.err, list.stderr)
	}
	if !strings.Contains(list.stdout, "module_id,label,module_kind,module_note,branch_name,worktree_dir") {
		t.Fatalf("unexpected helper --list output:\n%s", list.stdout)
	}
	if !strings.Contains(list.stdout, "auth_middleware,Auth and Middleware") {
		t.Fatalf("expected auth_middleware row in helper --list output:\n%s", list.stdout)
	}
}
