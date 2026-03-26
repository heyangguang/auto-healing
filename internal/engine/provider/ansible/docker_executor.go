package ansible

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/pkg/logger"
)

const (
	DefaultDockerImage   = "auto-healing/ansible-executor:latest"
	DefaultDockerTimeout = 30 * time.Minute
	dockerStopTimeoutSec = "5"
)

// DockerExecutor Docker 执行器
type DockerExecutor struct {
	image   string
	timeout time.Duration
}

// NewDockerExecutor 创建 Docker 执行器
// 配置优先级：config.yaml > 环境变量 > 默认值
func NewDockerExecutor() *DockerExecutor {
	image := ""
	timeout := DefaultDockerTimeout

	// 1. 从全局配置读取（config.yaml 里的 ansible 节）
	if cfg := appconfig.GetConfig(); cfg != nil && cfg.Ansible.ExecutorImage != "" {
		image = cfg.Ansible.ExecutorImage
		if cfg.Ansible.TimeoutMinutes > 0 {
			timeout = time.Duration(cfg.Ansible.TimeoutMinutes) * time.Minute
		}
	}

	// 2. 环境变量覆盖（向前兼容）
	if envImage := os.Getenv("ANSIBLE_EXECUTOR_IMAGE"); envImage != "" {
		image = envImage
	}
	if timeoutStr := os.Getenv("ANSIBLE_EXECUTOR_TIMEOUT"); timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = parsed
		}
	}

	// 3. 最终默认值
	if image == "" {
		image = DefaultDockerImage
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
		if err := WriteAnsibleCfg(req.WorkDir, nil); err != nil {
			return buildStreamingStartFailure(startedAt, fmt.Errorf("写入 ansible.cfg 失败: %w", err)), err
		}
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
func (e *DockerExecutor) stopContainer(containerName string) error {
	logger.Exec("DOCKER").Warn("正在停止容器: %s", containerName)
	// 使用 5 秒超时停止容器
	stopCmd := exec.Command("docker", "stop", "-t", dockerStopTimeoutSec, containerName)
	if err := stopCmd.Run(); err != nil {
		logger.Exec("DOCKER").Warn("停止容器失败，尝试强制 kill: %v", err)
		killCmd := exec.Command("docker", "kill", containerName)
		if killErr := killCmd.Run(); killErr != nil {
			logger.Exec("DOCKER").Error("强制 kill 容器失败: %s | stop_err=%v kill_err=%v", containerName, err, killErr)
			return fmt.Errorf("停止容器失败: %w；强制 kill 失败: %v", err, killErr)
		}
		logger.Exec("DOCKER").Warn("容器已强制 kill: %s", containerName)
		return nil
	}
	logger.Exec("DOCKER").Info("容器已停止: %s", containerName)
	return nil
}
