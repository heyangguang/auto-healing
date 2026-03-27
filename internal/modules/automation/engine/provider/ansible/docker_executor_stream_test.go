package ansible

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDockerExecutorStreamingStartFailureIsReturned(t *testing.T) {
	t.Setenv("PATH", "")

	executor := &DockerExecutor{image: "test-image"}
	result, err := executor.Execute(context.Background(), &ExecuteRequest{
		PlaybookPath: "site.yml",
		WorkDir:      t.TempDir(),
		LogCallback:  func(string, string, string) {},
	})

	if err == nil {
		t.Fatal("expected docker streaming start error")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestDockerExecutorStreamStdoutKeepsEOFLine(t *testing.T) {
	executor := &DockerExecutor{}
	req := &ExecuteRequest{}
	var output bytes.Buffer
	done := make(chan struct{})
	var messages []string
	req.LogCallback = func(level, stage, message string) {
		messages = append(messages, message)
	}

	executor.streamStdout(req, strings.NewReader("tail-without-newline"), &output, done)
	<-done

	if got := output.String(); got != "tail-without-newline" {
		t.Fatalf("output = %q, want final EOF line to be preserved", got)
	}
	if len(messages) != 1 || messages[0] != "tail-without-newline" {
		t.Fatalf("callback messages = %#v", messages)
	}
}

func TestDockerExecutorStreamStderrKeepsEOFLine(t *testing.T) {
	executor := &DockerExecutor{}
	var stderr bytes.Buffer
	done := make(chan struct{})
	req := &ExecuteRequest{}
	var messages []string
	req.LogCallback = func(level, stage, message string) {
		messages = append(messages, message)
	}

	executor.streamStderr(req, strings.NewReader("stderr-tail"), &stderr, done)
	<-done

	if got := stderr.String(); got != "stderr-tail" {
		t.Fatalf("stderr = %q, want final EOF line to be preserved", got)
	}
	if len(messages) != 1 || messages[0] != "stderr-tail" {
		t.Fatalf("callback messages = %#v", messages)
	}
}
