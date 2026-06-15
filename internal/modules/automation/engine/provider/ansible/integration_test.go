package ansible

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestLocalExecutorIntegrationPreparedWorkspaceAndStreaming(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "play.yml"), []byte("---\n"), 0600); err != nil {
		t.Fatalf("write playbook: %v", err)
	}

	workspaceBase := t.TempDir()
	t.Setenv("ANSIBLE_WORKSPACE_DIR", workspaceBase)
	manager := NewWorkspaceManager()
	workDir, cleanup, err := manager.PrepareWorkspace(uuid.New(), repoDir)
	if err != nil {
		t.Fatalf("PrepareWorkspace() error = %v", err)
	}
	defer cleanup()

	if _, err := WriteInventoryFile(workDir, GenerateInventory("localhost", "targets", nil)); err != nil {
		t.Fatalf("WriteInventoryFile() error = %v", err)
	}

	scriptPath := filepath.Join(t.TempDir(), "fake-ansible.sh")
	script := `#!/bin/sh
inv=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-i" ]; then
    inv="$2"
    shift 2
    continue
  fi
  shift
done
printf 'pwd=%s\n' "$PWD"
printf 'inventory=%s\n' "$inv"
cat "$inv"
printf 'stderr-line\n' >&2
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake ansible: %v", err)
	}

	var (
		mu       sync.Mutex
		messages []string
	)
	executor := &LocalExecutor{ansiblePath: scriptPath}
	result, err := executor.Execute(context.Background(), &ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      workDir,
		Inventory:    "inventory.ini",
		LogCallback: func(level, stage, message string) {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, message)
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(messages, "\n")
	combinedOutput := strings.Join([]string{joined, result.Stdout, result.Stderr}, "\n")
	if !strings.Contains(combinedOutput, "pwd="+workDir) {
		t.Fatalf("execution output missing working directory: messages=%v stdout=%q stderr=%q", messages, result.Stdout, result.Stderr)
	}
	if !strings.Contains(combinedOutput, "[targets]") || !strings.Contains(combinedOutput, "localhost") {
		t.Fatalf("execution output missing inventory content: messages=%v stdout=%q stderr=%q", messages, result.Stdout, result.Stderr)
	}
	if !strings.Contains(combinedOutput, "stderr-line") {
		t.Fatalf("stderr line missing from callback/result: messages=%v stderr=%q", messages, result.Stderr)
	}
	if !strings.Contains(combinedOutput, "inventory=") {
		t.Fatalf("execution output missing stdout content: messages=%v stdout=%q stderr=%q", messages, result.Stdout, result.Stderr)
	}
}
