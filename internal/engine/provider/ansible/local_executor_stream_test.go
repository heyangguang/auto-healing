package ansible

import (
	"bytes"
	"strings"
	"testing"
)

func TestLocalExecutorCollectStreamingOutputKeepsEOFLine(t *testing.T) {
	executor := NewLocalExecutor()
	req := &ExecuteRequest{}
	var output bytes.Buffer
	done := make(chan struct{})
	var messages []string
	req.LogCallback = func(level, stage, message string) {
		messages = append(messages, message)
	}

	executor.collectStreamingOutput(req, strings.NewReader("tail-without-newline"), &output, done)
	<-done

	if got := output.String(); got != "tail-without-newline" {
		t.Fatalf("output = %q, want final EOF line to be preserved", got)
	}
	if len(messages) != 1 || messages[0] != "tail-without-newline" {
		t.Fatalf("callback messages = %#v", messages)
	}
}
