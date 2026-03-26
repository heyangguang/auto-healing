package engine

import (
	"errors"
	"testing"

	"github.com/company/auto-healing/internal/engine/provider/ansible"
)

func TestNewExecutorSupportsKnownTypes(t *testing.T) {
	t.Helper()

	localExecutor, err := NewExecutor(ExecutorTypeLocal)
	if err != nil {
		t.Fatalf("NewExecutor(local) error = %v", err)
	}
	if _, ok := localExecutor.(*ansible.LocalExecutor); !ok {
		t.Fatalf("NewExecutor(local) returned %T", localExecutor)
	}

	dockerExecutor, err := NewExecutor(ExecutorTypeDocker)
	if err != nil {
		t.Fatalf("NewExecutor(docker) error = %v", err)
	}
	if _, ok := dockerExecutor.(*ansible.DockerExecutor); !ok {
		t.Fatalf("NewExecutor(docker) returned %T", dockerExecutor)
	}
}

func TestNewExecutorRejectsUnknownType(t *testing.T) {
	t.Helper()

	executor, err := NewExecutor("ssh")
	if executor != nil {
		t.Fatalf("expected nil executor for unsupported type, got %T", executor)
	}
	if !errors.Is(err, ErrUnsupportedExecutorType) {
		t.Fatalf("expected ErrUnsupportedExecutorType, got %v", err)
	}
}
