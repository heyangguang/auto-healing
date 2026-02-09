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
	DefaultAnsiblePath = "ansible-playbook"
)

// LocalExecutor 本地执行器
type LocalExecutor struct {
	ansiblePath string
}

// NewLocalExecutor 创建本地执行器
func NewLocalExecutor() *LocalExecutor {
	ansiblePath := os.Getenv("ANSIBLE_PLAYBOOK_PATH")
	if ansiblePath == "" {
		ansiblePath = DefaultAnsiblePath
	}
	return &LocalExecutor{ansiblePath: ansiblePath}
}

// Name 返回执行器名称
func (e *LocalExecutor) Name() string {
	return "local"
}

// Execute 执行 Ansible Playbook（支持实时日志流式输出）
func (e *LocalExecutor) Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResult, error) {
	startedAt := time.Now()

	// 确保 ansible.cfg 存在
	cfgPath := filepath.Join(req.WorkDir, "ansible.cfg")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		WriteAnsibleCfg(req.WorkDir, nil)
	}

	// 如果有日志回调，使用 script 命令包装实现实时输出
	if req.LogCallback != nil {
		return e.executeWithScript(ctx, req, startedAt)
	}

	// 否则使用缓冲方式
	return e.executeBuffered(ctx, req, startedAt)
}

// executeWithScript 使用 script 命令包装实现实时输出
// script 命令会强制程序以交互模式运行，禁用所有缓冲
func (e *LocalExecutor) executeWithScript(ctx context.Context, req *ExecuteRequest, startedAt time.Time) (*ExecuteResult, error) {
	args := e.buildArgs(req)

	// 构建完整的 ansible-playbook 命令
	ansibleCmd := e.ansiblePath + " " + strings.Join(args, " ")

	// 使用 script 命令包装，-q 安静模式，-c 指定命令
	// script -q /dev/null -c "command" 会强制实时输出
	cmd := exec.CommandContext(ctx, "script", "-q", "/dev/null", "-c", ansibleCmd)
	cmd.Dir = req.WorkDir
	cmd.Env = append(os.Environ(),
		"ANSIBLE_FORCE_COLOR=0",
		"ANSIBLE_NOCOLOR=1",
		"ANSIBLE_HOST_KEY_CHECKING=False",
		"PYTHONUNBUFFERED=1",
		"TERM=dumb", // 简单终端，避免控制字符
	)

	logger.Exec("ANSIBLE").Info("使用 script 命令实时输出模式")

	// 创建 stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		logger.Exec("ANSIBLE").Warn("script 命令启动失败，回退到普通模式: %v", err)
		return e.executeBuffered(ctx, req, startedAt)
	}

	// 收集输出
	var stdoutBuf bytes.Buffer

	// 实时读取输出
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}

		// 清理行（去除 \r\n 和控制字符）
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

	// 等待命令完成
	err = cmd.Wait()
	duration := time.Since(startedAt)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	stats := ParseStats(stdoutBuf.String())

	return &ExecuteResult{
		ExitCode:  exitCode,
		Stdout:    stdoutBuf.String(),
		Stderr:    "",
		Stats:     stats,
		StartedAt: startedAt,
		Duration:  duration,
	}, nil
}

// cleanControlChars 清理终端控制字符
func cleanControlChars(s string) string {
	// 移除 ANSI 转义序列
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
		// 跳过其他控制字符
		if s[i] < 32 && s[i] != '\t' {
			continue
		}
		result = append(result, s[i])
	}
	return string(result)
}

// detectLogLevel 根据 Ansible 输出判断日志级别
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

// executeBuffered 使用缓冲方式执行
func (e *LocalExecutor) executeBuffered(ctx context.Context, req *ExecuteRequest, startedAt time.Time) (*ExecuteResult, error) {
	args := e.buildArgs(req)

	cmd := exec.CommandContext(ctx, e.ansiblePath, args...)
	cmd.Dir = req.WorkDir
	cmd.Env = append(os.Environ(),
		"ANSIBLE_FORCE_COLOR=0",
		"ANSIBLE_NOCOLOR=1",
		"ANSIBLE_HOST_KEY_CHECKING=False",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
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

// buildArgs 构建命令行参数
func (e *LocalExecutor) buildArgs(req *ExecuteRequest) []string {
	var args []string

	playbookPath := req.PlaybookPath
	if !filepath.IsAbs(playbookPath) {
		playbookPath = filepath.Join(req.WorkDir, playbookPath)
	}
	args = append(args, playbookPath)

	if req.Inventory != "" {
		if _, err := os.Stat(req.Inventory); err == nil {
			args = append(args, "-i", req.Inventory)
		} else if strings.Contains(req.Inventory, " ") || strings.Contains(req.Inventory, "\n") {
			tmpFile, err := os.CreateTemp("", "ansible-inventory-*.ini")
			if err == nil {
				tmpFile.WriteString("[all]\n")
				tmpFile.WriteString(req.Inventory)
				tmpFile.Close()
				args = append(args, "-i", tmpFile.Name())
			}
		} else {
			args = append(args, "-i", req.Inventory+",")
		}
	}

	if len(req.ExtraVars) > 0 {
		jsonVars, _ := json.Marshal(req.ExtraVars)
		args = append(args, "--extra-vars", string(jsonVars))
	}

	if req.Limit != "" {
		args = append(args, "--limit", req.Limit)
	}

	if len(req.Tags) > 0 {
		args = append(args, "--tags", strings.Join(req.Tags, ","))
	}

	if len(req.SkipTags) > 0 {
		args = append(args, "--skip-tags", strings.Join(req.SkipTags, ","))
	}

	if req.Verbosity > 0 {
		verbosity := "-"
		for i := 0; i < req.Verbosity && i < 4; i++ {
			verbosity += "v"
		}
		args = append(args, verbosity)
	}

	if req.Become {
		args = append(args, "--become")
		if req.BecomeUser != "" {
			args = append(args, "--become-user", req.BecomeUser)
		}
	}

	return args
}

// CheckAnsibleInstalled 检查 ansible-playbook 是否已安装
func (e *LocalExecutor) CheckAnsibleInstalled() error {
	cmd := exec.Command(e.ansiblePath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ansible-playbook not found: %w", err)
	}
	return nil
}
