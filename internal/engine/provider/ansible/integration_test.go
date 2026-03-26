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
	if !strings.Contains(joined, "pwd="+workDir) {
		t.Fatalf("callback messages missing working directory output: %v", messages)
	}
	if !strings.Contains(joined, "[targets]") || !strings.Contains(joined, "localhost") {
		t.Fatalf("callback messages missing inventory content: %v", messages)
	}
	if !strings.Contains(joined, "stderr-line") {
		t.Fatalf("callback messages missing stderr line: %v", messages)
	}
	if !strings.Contains(joined, "inventory=") {
		t.Fatalf("callback messages missing stdout content: %v", messages)
	}
}
