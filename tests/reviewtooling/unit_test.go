package reviewtooling

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleCSVValidatorAcceptsCurrentModuleMap(t *testing.T) {
	root := repoRoot(t)
	validator := filepath.Join(root, "scripts/review/module_csv_validator.awk")
	modules := filepath.Join(root, "scripts/review/backend_modules.csv")

	result := runCommand(
		t,
		root,
		nil,
		"awk",
		"-F,",
		"-v", "expected="+moduleCSVHeader,
		"-f", validator,
		modules,
	)
	if result.err != nil {
		t.Fatalf("validator should accept backend_modules.csv: %v\nstderr:\n%s", result.err, result.stderr)
	}
}

func TestModuleCSVValidatorRejectsDuplicateSuffixes(t *testing.T) {
	root := repoRoot(t)
	validator := filepath.Join(root, "scripts/review/module_csv_validator.awk")
	csvPath := filepath.Join(t.TempDir(), "bad.csv")
	mustWriteFile(t, csvPath, strings.Join([]string{
		moduleCSVHeader,
		"auth_one,Auth One,business-core,note,dup,dup,internal/a.go,shared,focus",
		"auth_two,Auth Two,business-core,note,dup,dup,internal/b.go,shared,focus",
	}, "\n")+"\n", 0o644)

	result := runCommand(
		t,
		root,
		nil,
		"awk",
		"-F,",
		"-v", "expected="+moduleCSVHeader,
		"-f", validator,
		csvPath,
	)
	if result.err == nil {
		t.Fatal("validator should reject duplicate suffixes")
	}
	if !strings.Contains(result.stderr, "duplicate worktree_suffix") {
		t.Fatalf("expected duplicate worktree_suffix error, got stderr:\n%s", result.stderr)
	}
}

func TestSetupHelpWorksStandalone(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	target := filepath.Join(tmp, "setup_parallel_review.sh")
	source := filepath.Join(root, "scripts/review/setup_parallel_review.sh")
	mustWriteFile(t, target, mustReadFile(t, source), 0o755)

	result := runCommand(t, tmp, nil, "bash", target, "--help")
	if result.err != nil {
		t.Fatalf("standalone help should work: %v\nstderr:\n%s", result.err, result.stderr)
	}
	if !strings.Contains(result.stdout, "Required commands:") {
		t.Fatalf("expected help output to describe required commands, got:\n%s", result.stdout)
	}
}
