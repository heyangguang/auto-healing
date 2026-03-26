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

	"github.com/company/auto-healing/internal/pkg/logger"
)

// executeWithStreaming 使用流式方式执行 Docker，实时输出日志
func (e *DockerExecutor) executeWithStreaming(ctx context.Context, req *ExecuteRequest, startedAt time.Time) (*ExecuteResult, error) {
	containerName := e.generateContainerName(req.WorkDir)
	cmd, stdout, stderr, err := e.startStreamingCommand(ctx, req, containerName)
	if err != nil {
		return nil, err
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	cmdDone := make(chan error, 1)
	go e.streamStdout(req, stdout, &stdoutBuf, stdoutDone)
	go e.streamStderr(req, stderr, &stderrBuf, stderrDone)
	go func() { cmdDone <- cmd.Wait() }()
	return e.collectStreamingResult(ctx, startedAt, containerName, &stdoutBuf, &stderrBuf, stdoutDone, stderrDone, cmdDone)
}

func (e *DockerExecutor) startStreamingCommand(ctx context.Context, req *ExecuteRequest, containerName string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {
	args, err := e.buildDockerArgs(req, containerName)
	if err != nil {
		return nil, nil, nil, err
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	logger.Exec("DOCKER").Info("使用流式输出模式执行 Docker, 容器名: %s", containerName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("创建 stderr pipe 失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		logger.Exec("DOCKER").Warn("Docker 流式命令启动失败: %v", err)
		return nil, nil, nil, err
	}
	return cmd, stdout, stderr, nil
}

func (e *DockerExecutor) streamStdout(req *ExecuteRequest, stdout io.Reader, stdoutBuf *bytes.Buffer, readDone chan<- struct{}) {
	defer close(readDone)
	reader := bufio.NewReader(stdout)
	for {
		rawLine, err := reader.ReadString('\n')
		if rawLine != "" {
			line := cleanControlChars(strings.TrimRight(rawLine, "\r\n"))
			if line != "" {
				stdoutBuf.WriteString(line)
				if strings.HasSuffix(rawLine, "\n") {
					stdoutBuf.WriteByte('\n')
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

func (e *DockerExecutor) streamStderr(req *ExecuteRequest, stderr io.Reader, stderrBuf *bytes.Buffer, done chan<- struct{}) {
	defer close(done)
	reader := bufio.NewReader(stderr)
	for {
		rawLine, err := reader.ReadString('\n')
		if rawLine != "" {
			line := cleanControlChars(strings.TrimRight(rawLine, "\r\n"))
			if line != "" {
				stderrBuf.WriteString(line)
				if strings.HasSuffix(rawLine, "\n") {
					stderrBuf.WriteByte('\n')
				}
				if req != nil && req.LogCallback != nil {
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

func (e *DockerExecutor) collectStreamingResult(ctx context.Context, startedAt time.Time, containerName string, stdoutBuf, stderrBuf *bytes.Buffer, stdoutDone, stderrDone <-chan struct{}, cmdDone <-chan error) (*ExecuteResult, error) {
	select {
	case <-ctx.Done():
		return e.cancelStreamingExecution(startedAt, containerName, stdoutBuf, stderrBuf, stdoutDone, stderrDone, cmdDone, ctx.Err())
	case cmdErr := <-cmdDone:
		<-stdoutDone
		<-stderrDone
		return buildStreamingCommandResult(startedAt, stdoutBuf.String(), stderrBuf.String(), cmdErr)
	}
}

func (e *DockerExecutor) cancelStreamingExecution(startedAt time.Time, containerName string, stdoutBuf, stderrBuf *bytes.Buffer, stdoutDone, stderrDone <-chan struct{}, cmdDone <-chan error, cancelErr error) (*ExecuteResult, error) {
	logger.Exec("DOCKER").Warn("收到取消信号，正在停止容器...")
	if err := e.stopContainer(containerName); err != nil {
		logger.Exec("DOCKER").Error("取消执行时停止容器失败: %v", err)
	}
	<-cmdDone
	<-stdoutDone
	<-stderrDone
	return &ExecuteResult{
		ExitCode:  -1,
		Stdout:    stdoutBuf.String(),
		Stderr:    stderrBuf.String() + "\n执行被取消",
		StartedAt: startedAt,
		Duration:  time.Since(startedAt),
	}, cancelErr
}

func buildStreamingCommandResult(startedAt time.Time, stdout, stderr string, cmdErr error) (*ExecuteResult, error) {
	duration := time.Since(startedAt)
	exitCode, execErr := commandExitCode(cmdErr, stdout, stderr, startedAt, duration)
	if execErr != nil {
		return execErr, cmdErr
	}
	return &ExecuteResult{
		ExitCode:  exitCode,
		Stdout:    stdout,
		Stderr:    stderr,
		Stats:     ParseStats(stdout),
		StartedAt: startedAt,
		Duration:  duration,
	}, nil
}
