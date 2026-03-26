package ansible

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// executeWithStreaming 直接流式读取 stdout/stderr，避免静默回退到缓冲模式。
func (e *LocalExecutor) executeWithStreaming(ctx context.Context, req *ExecuteRequest, startedAt time.Time) (*ExecuteResult, error) {
	cmd, cleanup, err := e.buildStreamingCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	return e.runStreamingCommand(ctx, req, startedAt, cmd)
}

func (e *LocalExecutor) buildStreamingCommand(ctx context.Context, req *ExecuteRequest) (*exec.Cmd, func(), error) {
	args, cleanup, err := e.buildArgs(req)
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, e.ansiblePath, args...)
	cmd.Dir = req.WorkDir
	cmd.Env = buildExecuteEnv()
	return cmd, cleanup, nil
}

func (e *LocalExecutor) runStreamingCommand(ctx context.Context, req *ExecuteRequest, startedAt time.Time, cmd *exec.Cmd) (*ExecuteResult, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stderr pipe 失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 ansible-playbook 失败: %w", err)
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	cmdDone := make(chan error, 1)
	go e.collectStreamingOutput(req, stdout, &stdoutBuf, stdoutDone)
	go e.collectStreamingOutput(req, stderr, &stderrBuf, stderrDone)
	go func() { cmdDone <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		err := <-cmdDone
		<-stdoutDone
		<-stderrDone
		return cancelledStreamingResult(startedAt, &stdoutBuf, &stderrBuf, err), ctx.Err()
	case err := <-cmdDone:
		<-stdoutDone
		<-stderrDone
		return buildStreamingCommandResult(startedAt, stdoutBuf.String(), stderrBuf.String(), err)
	}
}

func (e *LocalExecutor) collectStreamingOutput(req *ExecuteRequest, stream io.Reader, output *bytes.Buffer, done chan<- struct{}) {
	defer close(done)

	reader := bufio.NewReader(stream)
	for {
		rawLine, err := reader.ReadString('\n')
		if rawLine != "" {
			line := cleanControlChars(strings.TrimRight(rawLine, "\r\n"))
			if line != "" {
				output.WriteString(line)
				if strings.HasSuffix(rawLine, "\n") {
					output.WriteByte('\n')
				}
				if req.LogCallback != nil {
					req.LogCallback(e.detectLogLevel(line), "execute", line)
				}
			}
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			return
		}
	}
}

func cancelledStreamingResult(startedAt time.Time, stdoutBuf, stderrBuf *bytes.Buffer, err error) *ExecuteResult {
	stderr := stderrBuf.String()
	if err != nil {
		stderr += "\n" + err.Error()
	}
	stderr += "\n执行被取消"
	return &ExecuteResult{
		ExitCode:  -1,
		Stdout:    stdoutBuf.String(),
		Stderr:    stderr,
		StartedAt: startedAt,
		Duration:  time.Since(startedAt),
	}
}

func cleanControlChars(s string) string {
	result := make([]byte, 0, len(s))
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				inEscape = false
			}
			continue
		}
		if s[i] < 32 && s[i] != '\t' {
			continue
		}
		result = append(result, s[i])
	}
	return string(result)
}

func (e *LocalExecutor) detectLogLevel(line string) string {
	switch {
	case strings.HasPrefix(line, "ok:"):
		return "ok"
	case strings.HasPrefix(line, "changed:"):
		return "changed"
	case strings.HasPrefix(line, "skipping:"):
		return "skipping"
	case strings.HasPrefix(line, "failed:"), strings.HasPrefix(line, "fatal:"):
		return "fatal"
	case strings.HasPrefix(line, "unreachable:"):
		return "unreachable"
	case strings.Contains(strings.ToLower(line), "error"):
		return "error"
	case strings.Contains(strings.ToLower(line), "warning"):
		return "warn"
	default:
		return "info"
	}
}
