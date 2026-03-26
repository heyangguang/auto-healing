package ansible

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	startedAt := time.Now()

	// 确保 ansible.cfg 存在
	cfgPath := filepath.Join(req.WorkDir, "ansible.cfg")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := WriteAnsibleCfg(req.WorkDir, nil); err != nil {
			return buildStreamingStartFailure(startedAt, fmt.Errorf("写入 ansible.cfg 失败: %w", err)), err
		}
	}

	if req.LogCallback != nil {
		return e.executeWithScript(ctx, req, startedAt)
	}

	return e.executeBuffered(ctx, req, startedAt)
}
