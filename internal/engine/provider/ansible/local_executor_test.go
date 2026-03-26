package ansible

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLocalExecutorExecuteDoesNotInterpretExtraVarsAsShell(t *testing.T) {
	t.Helper()

	workDir := t.TempDir()
	playbookPath := filepath.Join(workDir, "play.yml")
	if err := os.WriteFile(playbookPath, []byte("---\n"), 0600); err != nil {
		t.Fatalf("write playbook: %v", err)
	}

	argsPath := filepath.Join(workDir, "args.txt")
	scriptPath := filepath.Join(workDir, "fake-ansible.sh")
	script := "#!/bin/sh\nprintf '%s\n' \"$@\" > \"" + argsPath + "\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake ansible: %v", err)
	}

	probePath := filepath.Join(workDir, "probe")
	executor := &LocalExecutor{ansiblePath: scriptPath}
	_, err := executor.Execute(t.Context(), &ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      workDir,
		ExtraVars: map[string]any{
			"payload": "$(touch " + probePath + ")",
		},
		LogCallback: func(level, stage, message string) {},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, err := os.Stat(probePath); !os.IsNotExist(err) {
		t.Fatalf("probe file exists, shell payload was interpreted")
	}

	argsRaw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	if !strings.Contains(string(argsRaw), "$(touch "+probePath+")") {
		t.Fatalf("args missing literal payload: %s", string(argsRaw))
	}
}

func TestWriteInventoryAndConfigArePrivate(t *testing.T) {
	t.Helper()

	workDir := t.TempDir()
	path, err := WriteInventoryFile(workDir, "[all]\nlocalhost\n")
	if err != nil {
		t.Fatalf("WriteInventoryFile() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat inventory: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("inventory perm = %o, want 600", perm)
	}

	if err := WriteAnsibleCfg(workDir, nil); err != nil {
		t.Fatalf("WriteAnsibleCfg() error = %v", err)
	}
	cfgInfo, err := os.Stat(filepath.Join(workDir, "ansible.cfg"))
	if err != nil {
		t.Fatalf("stat ansible.cfg: %v", err)
	}
	if perm := cfgInfo.Mode().Perm(); perm != 0600 {
		t.Fatalf("ansible.cfg perm = %o, want 600", perm)
	}
}

func TestWorkspaceManagerPrepareWorkspaceCreatesPrivateDir(t *testing.T) {
	t.Helper()

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "play.yml"), []byte("---\n"), 0600); err != nil {
		t.Fatalf("write repo file: %v", err)
	}

	baseDir := t.TempDir()
	t.Setenv("ANSIBLE_WORKSPACE_DIR", baseDir)
	manager := NewWorkspaceManager()
	workDir, cleanup, err := manager.PrepareWorkspace(uuid.New(), repoDir)
	if err != nil {
		t.Fatalf("PrepareWorkspace() error = %v", err)
	}
	defer cleanup()

	info, err := os.Stat(workDir)
	if err != nil {
		t.Fatalf("stat work dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Fatalf("work dir perm = %o, want 700", perm)
	}
}

func TestWorkspaceManagerPrepareWorkspaceRejectsSymlink(t *testing.T) {
	t.Helper()

	repoDir := t.TempDir()
	targetFile := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(targetFile, []byte("secret"), 0600); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	linkPath := filepath.Join(repoDir, "leak")
	if err := os.Symlink(targetFile, linkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	baseDir := t.TempDir()
	t.Setenv("ANSIBLE_WORKSPACE_DIR", baseDir)
	manager := NewWorkspaceManager()
	_, _, err := manager.PrepareWorkspace(uuid.New(), repoDir)
	if err == nil {
		t.Fatal("PrepareWorkspace() error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "符号链接") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildArgsCreatesAndCleansTemporaryInventoryFile(t *testing.T) {
	t.Helper()

	executor := NewLocalExecutor()
	args, cleanup, err := executor.buildArgs(&ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      t.TempDir(),
		Inventory:    "host-a\nhost-b",
	})
	if err != nil {
		t.Fatalf("buildArgs() error = %v", err)
	}

	var inventoryPath string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-i" {
			inventoryPath = args[i+1]
			break
		}
	}
	if inventoryPath == "" {
		t.Fatal("expected temporary inventory path in args")
	}
	if _, err := os.Stat(inventoryPath); err != nil {
		t.Fatalf("temporary inventory does not exist: %v", err)
	}

	cleanup()

	if _, err := os.Stat(inventoryPath); !os.IsNotExist(err) {
		t.Fatalf("temporary inventory should be removed, stat err = %v", err)
	}
}

func TestBuildArgsRejectsUnmarshalableExtraVars(t *testing.T) {
	t.Helper()

	executor := NewLocalExecutor()
	_, _, err := executor.buildArgs(&ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      t.TempDir(),
		ExtraVars: map[string]any{
			"bad": make(chan int),
		},
	})
	if err == nil {
		t.Fatal("buildArgs() error = nil, want marshal error")
	}
	if !strings.Contains(err.Error(), "序列化 extra_vars 失败") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildDockerArgsRejectsUnmarshalableExtraVars(t *testing.T) {
	executor := NewDockerExecutor()
	_, err := executor.buildDockerArgs(&ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      t.TempDir(),
		ExtraVars: map[string]any{
			"bad": make(chan int),
		},
	}, "container")
	if err == nil {
		t.Fatal("buildDockerArgs() error = nil, want marshal error")
	}
	if !strings.Contains(err.Error(), "序列化 extra_vars 失败") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildDockerArgsReturnsErrorWhenInventoryFileWriteFails(t *testing.T) {
	executor := NewDockerExecutor()
	_, err := executor.buildDockerArgs(&ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      filepath.Join(t.TempDir(), "missing-dir"),
		Inventory:    "host-a\nhost-b",
	}, "container")
	if err == nil {
		t.Fatal("buildDockerArgs() error = nil, want inventory write failure")
	}
	if !strings.Contains(err.Error(), "写入 Docker inventory 文件失败") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildStreamingResultReturnsErrorForWaitFailure(t *testing.T) {
	result, err := buildStreamingResult(time.Now(), &bytes.Buffer{}, &bytes.Buffer{}, errors.New("wait failed"))
	if err == nil {
		t.Fatal("buildStreamingResult() error = nil, want wait failure")
	}
	if result.ExitCode != -1 {
		t.Fatalf("ExitCode = %d, want -1", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "wait failed") {
		t.Fatalf("Stderr = %q, want wait failure text", result.Stderr)
	}
}

func TestGenerateInventoryWithAuthRejectsUnsafeCredential(t *testing.T) {
	content := GenerateInventoryWithAuth([]HostCredential{
		{
			Host:     "host-a ansible_user=hacker",
			AuthType: "password",
			Username: "root",
			Password: "pass",
		},
	}, "targets")
	_, err := WriteInventoryFile(t.TempDir(), content)
	if err == nil {
		t.Fatal("WriteInventoryFile() error = nil, want inventory validation failure")
	}
}

func TestGenerateInventoryWithAuthAllowsPasswordWithSpaces(t *testing.T) {
	content := GenerateInventoryWithAuth([]HostCredential{
		{
			Host:     "host-a",
			AuthType: "password",
			Username: "root",
			Password: "pass with space",
		},
	}, "targets")
	if _, err := WriteInventoryFile(t.TempDir(), content); err != nil {
		t.Fatalf("WriteInventoryFile() error = %v", err)
	}
	if !strings.Contains(content, "ansible_ssh_pass='pass with space'") {
		t.Fatalf("content = %q, want quoted password", content)
	}
}

func TestGenerateInventoryWithAuthAllowsKeyPathWithSpaces(t *testing.T) {
	content := GenerateInventoryWithAuth([]HostCredential{
		{
			Host:     "host-a",
			AuthType: "ssh_key",
			KeyFile:  "/tmp/key path",
		},
	}, "targets")
	if _, err := WriteInventoryFile(t.TempDir(), content); err != nil {
		t.Fatalf("WriteInventoryFile() error = %v", err)
	}
	if !strings.Contains(content, "ansible_ssh_private_key_file='/tmp/key path'") {
		t.Fatalf("content = %q, want quoted key path", content)
	}
}

func TestWriteKeyFileRejectsPathTraversal(t *testing.T) {
	_, err := WriteKeyFile(t.TempDir(), "../id_rsa", "key")
	if err == nil {
		t.Fatal("WriteKeyFile() error = nil, want traversal rejection")
	}
}

func TestGenerateInventoryRejectsUnsafeHost(t *testing.T) {
	content := GenerateInventory("host-a\n[evil]", "targets", nil)
	_, err := WriteInventoryFile(t.TempDir(), content)
	if err == nil {
		t.Fatal("WriteInventoryFile() error = nil, want inventory validation failure")
	}
}

func TestLocalExecutorExecuteReturnsErrorWhenConfigWriteFails(t *testing.T) {
	executor := NewLocalExecutor()
	workDir := filepath.Join(t.TempDir(), "missing")
	_, err := executor.Execute(t.Context(), &ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      workDir,
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want ansible.cfg write failure")
	}
}

func TestDockerExecutorExecuteReturnsErrorWhenConfigWriteFails(t *testing.T) {
	executor := NewDockerExecutor()
	workDir := filepath.Join(t.TempDir(), "missing")
	_, err := executor.Execute(t.Context(), &ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      workDir,
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want ansible.cfg write failure")
	}
}
