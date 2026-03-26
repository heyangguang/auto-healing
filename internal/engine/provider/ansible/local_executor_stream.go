package ansible

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
)

// executeWithScript 使用 script 命令包装实现实时输出
func (e *LocalExecutor) executeWithScript(ctx context.Context, req *ExecuteRequest, startedAt time.Time) (*ExecuteResult, error) {
	cmd, cleanup, err := e.buildStreamingCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		logger.Exec("ANSIBLE").Warn("script 命令启动失败，回退到普通模式: %v", err)
		return e.executeBuffered(ctx, req, startedAt)
	}

	stdoutBuf := e.collectScriptOutput(req, stdout)
	err = cmd.Wait()
	return &ExecuteResult{
		ExitCode:  streamingExitCode(err),
		Stdout:    stdoutBuf.String(),
		Stderr:    "",
		Stats:     ParseStats(stdoutBuf.String()),
		StartedAt: startedAt,
		Duration:  time.Since(startedAt),
	}, nil
}

func (e *LocalExecutor) buildStreamingCommand(ctx context.Context, req *ExecuteRequest) (*exec.Cmd, func(), error) {
	args, cleanup, err := e.buildArgs(req)
	if err != nil {
		return nil, nil, err
	}
	ansibleCmd := buildShellCommand(append([]string{e.ansiblePath}, args...))
	cmd := exec.CommandContext(ctx, "script", "-q", "/dev/null", "-c", ansibleCmd)
	cmd.Dir = req.WorkDir
	cmd.Env = append(os.Environ(), "ANSIBLE_FORCE_COLOR=0", "ANSIBLE_NOCOLOR=1", "PYTHONUNBUFFERED=1", "TERM=dumb")
	logger.Exec("ANSIBLE").Info("使用 script 命令实时输出模式")
	return cmd, cleanup, nil
}

func (e *LocalExecutor) collectScriptOutput(req *ExecuteRequest, stdout io.Reader) bytes.Buffer {
	var stdoutBuf bytes.Buffer
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return stdoutBuf
			}
			return stdoutBuf
		}
		line = cleanControlChars(strings.TrimRight(line, "\r\n"))
		if line == "" {
			continue
		}
		stdoutBuf.WriteString(line + "\n")
		if req.LogCallback != nil {
			req.LogCallback(e.detectLogLevel(line), "execute", line)
		}
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
