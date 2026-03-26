package ansible

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func deriveExecuteContext(parent context.Context, requestTimeout, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	timeout := requestTimeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}

func ensureAnsibleCfg(workDir string) error {
	cfgPath := filepath.Join(workDir, "ansible.cfg")
	if _, err := os.Stat(cfgPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查 ansible.cfg 失败: %w", err)
	}
	if err := WriteAnsibleCfg(workDir, nil); err != nil {
		return fmt.Errorf("写入 ansible.cfg 失败: %w", err)
	}
	return nil
}
