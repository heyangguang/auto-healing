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
	cmd, stdout, stderr, err := e.startStreamingCommand(req, containerName)
	if err != nil {
		return buildStreamingStartFailure(startedAt, err), err
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	readDone := make(chan struct{})
	cmdDone := make(chan error, 1)
	go e.streamStdout(req, stdout, &stdoutBuf, readDone)
	go e.streamStderr(stderr, &stderrBuf)
	go func() { cmdDone <- cmd.Wait() }()
	return e.collectStreamingResult(ctx, req, startedAt, containerName, &stdoutBuf, &stderrBuf, readDone, cmdDone)
}

func (e *DockerExecutor) startStreamingCommand(req *ExecuteRequest, containerName string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {
	args, err := e.buildDockerArgs(req, containerName)
	if err != nil {
		return nil, nil, nil, err
	}
	cmd := exec.Command("docker", args...)
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
		return nil, nil, nil, fmt.Errorf("启动 Docker 命令失败: %w", err)
	}
	return cmd, stdout, stderr, nil
}

func buildStreamingStartFailure(startedAt time.Time, err error) *ExecuteResult {
	return &ExecuteResult{
		ExitCode:  -1,
		Stderr:    err.Error(),
		StartedAt: startedAt,
		Duration:  time.Since(startedAt),
	}
}

func (e *DockerExecutor) streamStdout(req *ExecuteRequest, stdout io.Reader, stdoutBuf *bytes.Buffer, readDone chan<- struct{}) {
	defer close(readDone)
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			return
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

func (e *DockerExecutor) streamStderr(stderr io.Reader, stderrBuf *bytes.Buffer) {
	reader := bufio.NewReader(stderr)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		stderrBuf.WriteString(line)
	}
}

func (e *DockerExecutor) collectStreamingResult(ctx context.Context, req *ExecuteRequest, startedAt time.Time, containerName string, stdoutBuf, stderrBuf *bytes.Buffer, readDone <-chan struct{}, cmdDone <-chan error) (*ExecuteResult, error) {
	select {
	case <-ctx.Done():
		return e.cancelStreamingExecution(startedAt, containerName, stdoutBuf, stderrBuf, cmdDone, ctx.Err())
	case cmdErr := <-cmdDone:
		<-readDone
		return buildStreamingResult(startedAt, stdoutBuf, stderrBuf, cmdErr)
	}
}

func (e *DockerExecutor) cancelStreamingExecution(startedAt time.Time, containerName string, stdoutBuf, stderrBuf *bytes.Buffer, cmdDone <-chan error, cancelErr error) (*ExecuteResult, error) {
	logger.Exec("DOCKER").Warn("收到取消信号，正在停止容器...")
	if err := e.stopContainer(containerName); err != nil {
		logger.Exec("DOCKER").Error("取消执行时停止容器失败: %v", err)
	}
	<-cmdDone
	return &ExecuteResult{
		ExitCode:  -1,
		Stdout:    stdoutBuf.String(),
		Stderr:    stderrBuf.String() + "\n执行被取消",
		StartedAt: startedAt,
		Duration:  time.Since(startedAt),
	}, cancelErr
}

func buildStreamingResult(startedAt time.Time, stdoutBuf, stderrBuf *bytes.Buffer, cmdErr error) (*ExecuteResult, error) {
	result := &ExecuteResult{
		ExitCode:  streamingExitCode(cmdErr),
		Stdout:    stdoutBuf.String(),
		Stderr:    stderrBuf.String(),
		Stats:     ParseStats(stdoutBuf.String()),
		StartedAt: startedAt,
		Duration:  time.Since(startedAt),
	}
	if cmdErr == nil {
		return result, nil
	}
	if _, ok := cmdErr.(*exec.ExitError); ok {
		return result, nil
	}
	if result.Stderr == "" {
		result.Stderr = cmdErr.Error()
	} else {
		result.Stderr += "\n" + cmdErr.Error()
	}
	return result, cmdErr
}

func streamingExitCode(cmdErr error) int {
	if cmdErr == nil {
		return 0
	}
	if exitErr, ok := cmdErr.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
