package reviewtooling

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const moduleCSVHeader = "module_id,label,module_kind,module_note,worktree_suffix,branch_suffix,paths,shared_touchpoints,focus"

type commandResult struct {
	stdout string
	stderr string
	err    error
}

func repoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func runCommand(t *testing.T, dir string, env map[string]string, name string, args ...string) commandResult {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), envPairs(env)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func envPairs(env map[string]string) []string {
	pairs := make([]string, 0, len(env))
	for key, value := range env {
		pairs = append(pairs, key+"="+value)
	}
	return pairs
}

func mustWriteFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func setupSession(t *testing.T, reviewRoot string, session string) string {
	t.Helper()
	root := repoRoot(t)
	script := filepath.Join(root, "scripts/review/setup_parallel_review.sh")
	result := runCommand(t, root, map[string]string{"REVIEW_ROOT": reviewRoot}, "bash", script, session)
	if result.err != nil {
		t.Fatalf("setup session failed: %v\nstderr:\n%s", result.err, result.stderr)
	}
	return filepath.Join(reviewRoot, session)
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}
