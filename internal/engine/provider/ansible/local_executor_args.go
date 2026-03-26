package ansible

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/company/auto-healing/internal/pkg/logger"
)

// buildArgs 构建命令行参数
func (e *LocalExecutor) buildArgs(req *ExecuteRequest) ([]string, func(), error) {
	args := []string{resolveLocalPlaybookPath(req.WorkDir, req.PlaybookPath)}
	cleanup := func() {}

	inventoryArgs, inventoryCleanup, err := buildLocalInventoryArgs(req.Inventory)
	if err != nil {
		return nil, nil, err
	}
	args = append(args, inventoryArgs...)
	if inventoryCleanup != nil {
		cleanup = inventoryCleanup
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
		args = append(args, buildVerbosityFlag(req.Verbosity))
	}
	if req.Become {
		args = append(args, "--become")
		if req.BecomeUser != "" {
			args = append(args, "--become-user", req.BecomeUser)
		}
	}
	return args, cleanup, nil
}

func resolveLocalPlaybookPath(workDir, playbookPath string) string {
	if filepath.IsAbs(playbookPath) {
		return playbookPath
	}
	return filepath.Join(workDir, playbookPath)
}

func buildLocalInventoryArgs(inventory string) ([]string, func(), error) {
	if inventory == "" {
		return nil, nil, nil
	}
	if _, err := os.Stat(inventory); err == nil {
		return []string{"-i", inventory}, nil, nil
	}
	if strings.Contains(inventory, " ") || strings.Contains(inventory, "\n") {
		return buildTemporaryInventory(inventory)
	}
	return []string{"-i", inventory + ","}, nil, nil
}

func buildTemporaryInventory(inventory string) ([]string, func(), error) {
	tmpFile, err := os.CreateTemp("", "ansible-inventory-*.ini")
	if err != nil {
		return nil, nil, fmt.Errorf("创建临时 inventory 文件失败: %w", err)
	}
	if _, err := tmpFile.WriteString("[all]\n"); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("写入临时 inventory 文件失败: %w", err)
	}
	if _, err := tmpFile.WriteString(inventory); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("写入临时 inventory 文件失败: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("关闭临时 inventory 文件失败: %w", err)
	}
	tmpInventoryPath := tmpFile.Name()
	cleanup := func() {
		if err := os.Remove(tmpInventoryPath); err != nil && !os.IsNotExist(err) {
			logger.Exec("ANSIBLE").Warn("清理临时 inventory 文件失败: %v", err)
		}
	}
	return []string{"-i", tmpInventoryPath}, cleanup, nil
}

func buildVerbosityFlag(level int) string {
	verbosity := "-"
	for i := 0; i < level && i < 4; i++ {
		verbosity += "v"
	}
	return verbosity
}

// CheckAnsibleInstalled 检查 ansible-playbook 是否已安装
func (e *LocalExecutor) CheckAnsibleInstalled() error {
	cmd := exec.Command(e.ansiblePath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ansible-playbook not found: %w", err)
	}
	return nil
}

func emitBufferedLogs(stdout, stderr string, callback LogCallback, detect func(string) string) {
	if callback == nil {
		return
	}
	emit := func(output string) {
		for _, line := range strings.Split(output, "\n") {
			line = cleanControlChars(strings.TrimSpace(line))
			if line == "" {
				continue
			}
			callback(detect(line), "execute", line)
		}
	}
	emit(stdout)
	emit(stderr)
}

func buildShellCommand(argv []string) string {
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(arg string) string {
	quote := string([]byte{39})
	if arg == "" {
		return quote + quote
	}
	escaped := strings.ReplaceAll(arg, quote, quote+`"`+quote+`"`+quote)
	return quote + escaped + quote
}
