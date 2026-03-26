package ansible

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
)

// executeBuffered 使用缓冲方式执行（原有逻辑，也支持取消）
func (e *DockerExecutor) executeBuffered(ctx context.Context, req *ExecuteRequest, startedAt time.Time) (*ExecuteResult, error) {
	containerName := e.generateContainerName(req.WorkDir)
	cmd := exec.Command("docker", e.buildDockerArgs(req, containerName)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return &ExecuteResult{ExitCode: -1, Stderr: err.Error(), StartedAt: startedAt, Duration: time.Since(startedAt)}, err
	}
	cmdDone := make(chan error, 1)
	go func() { cmdDone <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		return e.cancelBufferedExecution(startedAt, containerName, &stdout, &stderr, cmdDone, ctx.Err())
	case err := <-cmdDone:
		return buildBufferedResult(startedAt, &stdout, &stderr, err)
	}
}

func (e *DockerExecutor) cancelBufferedExecution(startedAt time.Time, containerName string, stdout, stderr *bytes.Buffer, cmdDone <-chan error, cancelErr error) (*ExecuteResult, error) {
	logger.Exec("DOCKER").Warn("收到取消信号，正在停止容器...")
	if stopErr := e.stopContainer(containerName); stopErr != nil {
		logger.Exec("DOCKER").Error("取消执行时停止容器失败: %v", stopErr)
	}
	<-cmdDone
	return &ExecuteResult{
		ExitCode:  -1,
		Stdout:    stdout.String(),
		Stderr:    stderr.String() + "\n执行被取消",
		StartedAt: startedAt,
		Duration:  time.Since(startedAt),
	}, cancelErr
}

func buildBufferedResult(startedAt time.Time, stdout, stderr *bytes.Buffer, err error) (*ExecuteResult, error) {
	duration := time.Since(startedAt)
	if err == nil {
		return &ExecuteResult{
			ExitCode:  0,
			Stdout:    stdout.String(),
			Stderr:    stderr.String(),
			Stats:     ParseStats(stdout.String()),
			StartedAt: startedAt,
			Duration:  duration,
		}, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return &ExecuteResult{
			ExitCode:  exitErr.ExitCode(),
			Stdout:    stdout.String(),
			Stderr:    stderr.String(),
			Stats:     ParseStats(stdout.String()),
			StartedAt: startedAt,
			Duration:  duration,
		}, nil
	}
	return &ExecuteResult{
		ExitCode:  -1,
		Stdout:    stdout.String(),
		Stderr:    stderr.String() + "\n" + err.Error(),
		StartedAt: startedAt,
		Duration:  duration,
	}, err
}

func (e *DockerExecutor) detectLogLevel(line string) string {
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
