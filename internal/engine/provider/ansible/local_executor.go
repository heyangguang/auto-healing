package ansible

import (
	"context"
	"os"
	"time"
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
	execCtx, cancel := deriveExecuteContext(ctx, req.Timeout, 0)
	defer cancel()

	startedAt := time.Now()
	if err := ensureAnsibleCfg(req.WorkDir); err != nil {
		return nil, err
	}
	if req.LogCallback != nil {
		return e.executeWithStreaming(execCtx, req, startedAt)
	}

	// 避免把参数回拼成 shell 命令，这里统一走 argv-safe 的执行路径。
	return e.executeBuffered(execCtx, req, startedAt)
}
