package ansible

import (
	"strings"
	"testing"
)

func TestLocalBuildArgsRejectsPlaybookOutsideWorkDir(t *testing.T) {
	executor := NewLocalExecutor()
	_, _, err := executor.buildArgs(&ExecuteRequest{
		PlaybookPath: "../outside.yml",
		WorkDir:      t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "playbook 必须位于工作目录内") {
		t.Fatalf("expected playbook boundary error, got %v", err)
	}
}

func TestDockerBuildArgsRejectsPlaybookOutsideWorkDir(t *testing.T) {
	executor := &DockerExecutor{image: "test-image"}
	_, err := executor.buildDockerArgs(&ExecuteRequest{
		PlaybookPath: "../outside.yml",
		WorkDir:      t.TempDir(),
	}, "container")
	if err == nil || !strings.Contains(err.Error(), "playbook 必须位于工作目录内") {
		t.Fatalf("expected playbook boundary error, got %v", err)
	}
}

func TestDockerBuildArgsDoesNotForceTTY(t *testing.T) {
	executor := &DockerExecutor{image: "test-image"}
	args, err := executor.buildDockerArgs(&ExecuteRequest{
		PlaybookPath: "play.yml",
		WorkDir:      t.TempDir(),
	}, "container")
	if err != nil {
		t.Fatalf("buildDockerArgs() error = %v", err)
	}
	for _, arg := range args {
		if arg == "-t" {
			t.Fatalf("expected docker args without forced tty, got %v", args)
		}
	}
}
