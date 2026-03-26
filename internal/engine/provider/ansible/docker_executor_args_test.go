package ansible

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerBuildArgsRejectsUnmarshalableExtraVars(t *testing.T) {
	t.Helper()

	executor := &DockerExecutor{image: "test-image"}
	_, err := executor.buildDockerArgs(&ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      t.TempDir(),
		ExtraVars: map[string]any{
			"bad": make(chan int),
		},
	}, "container")
	if err == nil || !strings.Contains(err.Error(), "序列化 extra_vars 失败") {
		t.Fatalf("expected extra_vars marshal error, got %v", err)
	}
}

func TestDockerBuildArgsUsesWorkdirRelativeInventoryFile(t *testing.T) {
	t.Helper()

	workDir := t.TempDir()
	inventoryPath := filepath.Join(workDir, "inventory.ini")
	if _, err := WriteInventoryFile(workDir, "[all]\nlocalhost\n"); err != nil {
		t.Fatalf("WriteInventoryFile() error = %v", err)
	}

	executor := &DockerExecutor{image: "test-image"}
	args, err := executor.buildDockerArgs(&ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      workDir,
		Inventory:    inventoryPath,
	}, "container")
	if err != nil {
		t.Fatalf("buildDockerArgs() error = %v", err)
	}
	if !containsArgPair(args, "-i", "/workspace/inventory.ini") {
		t.Fatalf("expected docker inventory file arg, got %v", args)
	}
}

func TestDockerBuildArgsRejectsInventoryOutsideWorkdir(t *testing.T) {
	t.Helper()

	workDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideInventory := filepath.Join(outsideDir, "inventory.ini")
	if _, err := WriteInventoryFile(outsideDir, "[all]\nlocalhost\n"); err != nil {
		t.Fatalf("WriteInventoryFile(outside) error = %v", err)
	}

	executor := &DockerExecutor{image: "test-image"}
	_, err := executor.buildDockerArgs(&ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      workDir,
		Inventory:    outsideInventory,
	}, "container")
	if err == nil || !strings.Contains(err.Error(), "inventory 文件必须位于工作目录内") {
		t.Fatalf("expected out-of-workdir inventory error, got %v", err)
	}
}

func TestDockerBuildArgsTreatsInventoryPathWithSpacesAsFile(t *testing.T) {
	t.Helper()

	workDir := t.TempDir()
	inventoryPath := filepath.Join(workDir, "inventory with space.ini")
	if err := os.WriteFile(inventoryPath, []byte("[all]\nlocalhost\n"), 0600); err != nil {
		t.Fatalf("write inventory with spaces: %v", err)
	}

	executor := &DockerExecutor{image: "test-image"}
	args, err := executor.buildDockerArgs(&ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      workDir,
		Inventory:    inventoryPath,
	}, "container")
	if err != nil {
		t.Fatalf("buildDockerArgs() error = %v", err)
	}
	if !containsArgPair(args, "-i", "/workspace/inventory with space.ini") {
		t.Fatalf("expected inventory path with spaces to be treated as file, got %v", args)
	}
}

func containsArgPair(args []string, key, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}
