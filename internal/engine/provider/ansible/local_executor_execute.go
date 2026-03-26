package ansible

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"time"
)

// executeBuffered 使用缓冲方式执行
func (e *LocalExecutor) executeBuffered(ctx context.Context, req *ExecuteRequest, startedAt time.Time) (*ExecuteResult, error) {
	args, cleanup, err := e.buildArgs(req)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	cmd := exec.CommandContext(ctx, e.ansiblePath, args...)
	cmd.Dir = req.WorkDir
	cmd.Env = append(os.Environ(), "ANSIBLE_FORCE_COLOR=0", "ANSIBLE_NOCOLOR=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	duration := time.Since(startedAt)
	exitCode, execErr := commandExitCode(err, stdout.String(), stderr.String(), startedAt, duration)
	if execErr != nil {
		return execErr, err
	}

	stats := ParseStats(stdout.String())
	if req.LogCallback != nil {
		emitBufferedLogs(stdout.String(), stderr.String(), req.LogCallback, e.detectLogLevel)
	}
	return &ExecuteResult{
		ExitCode:  exitCode,
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		Stats:     stats,
		StartedAt: startedAt,
		Duration:  duration,
	}, nil
}

func commandExitCode(err error, stdout, stderr string, startedAt time.Time, duration time.Duration) (int, *ExecuteResult) {
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), nil
	}
	return 0, &ExecuteResult{
		ExitCode:  -1,
		Stdout:    stdout,
		Stderr:    stderr + "\n" + err.Error(),
		StartedAt: startedAt,
		Duration:  duration,
	}
}
