package ansible

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
)

const (
	DefaultDockerImage   = "auto-healing/ansible-executor:latest"
	DefaultDockerTimeout = 30 * time.Minute
)

// DockerExecutor Docker 执行器
type DockerExecutor struct {
	image   string
	timeout time.Duration
}

// NewDockerExecutor 创建 Docker 执行器
func NewDockerExecutor() *DockerExecutor {
	image := os.Getenv("ANSIBLE_EXECUTOR_IMAGE")
	if image == "" {
		image = DefaultDockerImage
	}

	timeout := DefaultDockerTimeout
	if timeoutStr := os.Getenv("ANSIBLE_EXECUTOR_TIMEOUT"); timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = parsed
		}
	}

	return &DockerExecutor{
		image:   image,
		timeout: timeout,
	}
}

// Name 返回执行器名称
func (e *DockerExecutor) Name() string {
	return "docker"
}

// Execute 执行 Ansible Playbook（支持实时日志流式输出）
func (e *DockerExecutor) Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResult, error) {
	startedAt := time.Now()

	// 确保 ansible.cfg 存在
	cfgPath := filepath.Join(req.WorkDir, "ansible.cfg")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		WriteAnsibleCfg(req.WorkDir, nil)
	}

	// 如果有日志回调，使用流式方式
	if req.LogCallback != nil {
		return e.executeWithStreaming(ctx, req, startedAt)
	}

	// 否则使用缓冲方式
	return e.executeBuffered(ctx, req, startedAt)
}

// generateContainerName 根据工作目录生成容器名称
func (e *DockerExecutor) generateContainerName(workDir string) string {
	// 从工作目录路径中提取 runID 作为容器名称
	// 工作目录格式: /tmp/ansible_workspace_<runID>
	base := filepath.Base(workDir)
	if strings.HasPrefix(base, "ansible_workspace_") {
		return "ansible-exec-" + strings.TrimPrefix(base, "ansible_workspace_")
	}
	// 回退：使用时间戳
	return fmt.Sprintf("ansible-exec-%d", time.Now().UnixNano())
}

// stopContainer 停止指定名称的容器
func (e *DockerExecutor) stopContainer(containerName string) {
	logger.Exec("DOCKER").Warn("正在停止容器: %s", containerName)
	// 使用 5 秒超时停止容器
	stopCmd := exec.Command("docker", "stop", "-t", "5", containerName)
	if err := stopCmd.Run(); err != nil {
		logger.Exec("DOCKER").Warn("停止容器失败 (可能已退出): %v", err)
	} else {
		logger.Exec("DOCKER").Info("容器已停止: %s", containerName)
	}
}

// executeWithStreaming 使用流式方式执行 Docker，实时输出日志
func (e *DockerExecutor) executeWithStreaming(ctx context.Context, req *ExecuteRequest, startedAt time.Time) (*ExecuteResult, error) {
	// 生成容器名称用于取消操作
	containerName := e.generateContainerName(req.WorkDir)
	args := e.buildDockerArgs(req, containerName)

	// 不使用 CommandContext，我们自己处理取消
	cmd := exec.Command("docker", args...)

	logger.Exec("DOCKER").Info("使用流式输出模式执行 Docker, 容器名: %s", containerName)

	// 创建 stdout 和 stderr pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stderr pipe 失败: %w", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		logger.Exec("DOCKER").Warn("Docker 命令启动失败，回退到缓冲模式: %v", err)
		return e.executeBuffered(ctx, req, startedAt)
	}

	// 收集输出
	var stdoutBuf, stderrBuf bytes.Buffer

	// 用于等待读取完成
	readDone := make(chan struct{})
	// 用于等待命令完成
	cmdDone := make(chan error, 1)

	// 读取并流式输出 stdout
	go func() {
		defer close(readDone)
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				break
			}

			// 清理行
			line = strings.TrimRight(line, "\r\n")
			line = cleanControlChars(line)
			if line == "" {
				continue
			}

			stdoutBuf.WriteString(line + "\n")

			// 调用回调实时输出
			if req.LogCallback != nil {
				logLevel := e.detectLogLevel(line)
				req.LogCallback(logLevel, "execute", line)
			}
		}
	}()

	// 读取 stderr（不流式输出，只收集）
	go func() {
		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			stderrBuf.WriteString(line)
		}
	}()

	// 等待命令完成
	go func() {
		cmdDone <- cmd.Wait()
	}()

	// 等待命令完成或 context 取消
	var cmdErr error
	select {
	case <-ctx.Done():
		// Context 被取消，停止容器
		logger.Exec("DOCKER").Warn("收到取消信号，正在停止容器...")
		e.stopContainer(containerName)
		// 等待命令退出
		cmdErr = <-cmdDone
		// 返回取消错误
		return &ExecuteResult{
			ExitCode:  -1,
			Stdout:    stdoutBuf.String(),
			Stderr:    stderrBuf.String() + "\n执行被取消",
			Stats:     nil,
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
		}, ctx.Err()
	case cmdErr = <-cmdDone:
		// 命令正常结束，等待读取完成
		<-readDone
	}

	duration := time.Since(startedAt)

	exitCode := 0
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	stats := ParseStats(stdoutBuf.String())

	return &ExecuteResult{
		ExitCode:  exitCode,
		Stdout:    stdoutBuf.String(),
		Stderr:    stderrBuf.String(),
		Stats:     stats,
		StartedAt: startedAt,
		Duration:  duration,
	}, nil
}

// executeBuffered 使用缓冲方式执行（原有逻辑，也支持取消）
func (e *DockerExecutor) executeBuffered(ctx context.Context, req *ExecuteRequest, startedAt time.Time) (*ExecuteResult, error) {
	// 生成容器名称用于取消操作
	containerName := e.generateContainerName(req.WorkDir)
	args := e.buildDockerArgs(req, containerName)

	// 不使用 CommandContext，我们自己处理取消
	cmd := exec.Command("docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 启动命令
	if err := cmd.Start(); err != nil {
		return &ExecuteResult{
			ExitCode:  -1,
			Stdout:    "",
			Stderr:    err.Error(),
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
		}, err
	}

	// 用于等待命令完成
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	// 等待命令完成或 context 取消
	var err error
	select {
	case <-ctx.Done():
		// Context 被取消，停止容器
		logger.Exec("DOCKER").Warn("收到取消信号，正在停止容器...")
		e.stopContainer(containerName)
		// 等待命令退出
		<-cmdDone
		// 返回取消错误
		return &ExecuteResult{
			ExitCode:  -1,
			Stdout:    stdout.String(),
			Stderr:    stderr.String() + "\n执行被取消",
			Stats:     nil,
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
		}, ctx.Err()
	case err = <-cmdDone:
		// 命令正常结束
	}

	duration := time.Since(startedAt)
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return &ExecuteResult{
				ExitCode:  -1,
				Stdout:    stdout.String(),
				Stderr:    stderr.String() + "\n" + err.Error(),
				StartedAt: startedAt,
				Duration:  duration,
			}, err
		}
	}

	stats := ParseStats(stdout.String())

	return &ExecuteResult{
		ExitCode:  exitCode,
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		Stats:     stats,
		StartedAt: startedAt,
		Duration:  duration,
	}, nil
}

// detectLogLevel 根据 Ansible 输出判断日志级别
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

// buildDockerArgs 构建 Docker 命令行参数
func (e *DockerExecutor) buildDockerArgs(req *ExecuteRequest, containerName string) []string {
	args := []string{
		"run",
		"--rm",
		"-t",                    // 分配伪终端，强制行缓冲实现实时输出
		"--name", containerName, // 容器名称，用于取消时停止
		"--network", "host", // 使用主机网络
	}

	// 挂载工作目录到容器
	args = append(args, "-v", fmt.Sprintf("%s:/workspace:ro", req.WorkDir))

	// 设置工作目录
	args = append(args, "-w", "/workspace")

	// 设置环境变量
	args = append(args,
		"-e", "ANSIBLE_FORCE_COLOR=0",
		"-e", "ANSIBLE_NOCOLOR=1",
		"-e", "ANSIBLE_HOST_KEY_CHECKING=False",
		"-e", "ANSIBLE_PYTHON_INTERPRETER=auto",
		"-e", "PYTHONUNBUFFERED=1", // 禁用 Python 输出缓冲，确保实时流式输出
		// 添加 RHEL/CentOS 的 platform-python 到回退列表
		"-e", "ANSIBLE_INTERPRETER_PYTHON_FALLBACK=python3.11,python3.10,python3.9,python3.8,python3.7,python3.6,/usr/bin/python3,/usr/libexec/platform-python,python2.7,/usr/bin/python,python",
	)

	// 镜像名称
	args = append(args, e.image)

	// Playbook 路径 (相对于 /workspace)
	playbookPath := "/workspace/" + req.PlaybookPath
	args = append(args, playbookPath)

	// Inventory
	if req.Inventory != "" {
		// 检查是否是文件路径
		if strings.HasPrefix(req.Inventory, req.WorkDir) {
			// 转换为容器内路径
			relativePath := strings.TrimPrefix(req.Inventory, req.WorkDir)
			args = append(args, "-i", "/workspace"+relativePath)
		} else if strings.Contains(req.Inventory, " ") || strings.Contains(req.Inventory, "\n") {
			// inventory 内容包含空格或换行，需要写入工作目录的文件
			inventoryPath := filepath.Join(req.WorkDir, "inventory.ini")
			if err := os.WriteFile(inventoryPath, []byte("[all]\n"+req.Inventory+"\n"), 0644); err == nil {
				args = append(args, "-i", "/workspace/inventory.ini")
			} else {
				// 回退：直接作为主机列表传递
				args = append(args, "-i", req.Inventory+",")
			}
		} else {
			// 简单的主机名，作为逗号分隔的主机列表
			args = append(args, "-i", req.Inventory+",")
		}
	}

	// Extra vars
	if len(req.ExtraVars) > 0 {
		jsonVars, _ := json.Marshal(req.ExtraVars)
		args = append(args, "--extra-vars", string(jsonVars))
	}

	// Limit
	if req.Limit != "" {
		args = append(args, "--limit", req.Limit)
	}

	// Tags
	if len(req.Tags) > 0 {
		args = append(args, "--tags", strings.Join(req.Tags, ","))
	}

	// Skip tags
	if len(req.SkipTags) > 0 {
		args = append(args, "--skip-tags", strings.Join(req.SkipTags, ","))
	}

	// Verbosity
	if req.Verbosity > 0 {
		verbosity := "-"
		for i := 0; i < req.Verbosity && i < 4; i++ {
			verbosity += "v"
		}
		args = append(args, verbosity)
	}

	// Become
	if req.Become {
		args = append(args, "--become")
		if req.BecomeUser != "" {
			args = append(args, "--become-user", req.BecomeUser)
		}
	}

	return args
}

// CheckDockerInstalled 检查 Docker 是否已安装
func (e *DockerExecutor) CheckDockerInstalled() error {
	cmd := exec.Command("docker", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker not found: %w", err)
	}
	return nil
}

// CheckImageExists 检查镜像是否存在
func (e *DockerExecutor) CheckImageExists() error {
	cmd := exec.Command("docker", "image", "inspect", e.image)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("image %s not found: %w", e.image, err)
	}
	return nil
}
