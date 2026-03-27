package ansible

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

const (
	DefaultWorkspaceBase = "/tmp/auto-healing/exec"
)

// WorkspaceManager 工作空间管理器
type WorkspaceManager struct {
	baseDir string
}

// NewWorkspaceManager 创建工作空间管理器
func NewWorkspaceManager() *WorkspaceManager {
	baseDir := os.Getenv("ANSIBLE_WORKSPACE_DIR")
	if baseDir == "" {
		baseDir = DefaultWorkspaceBase
	}
	return &WorkspaceManager{baseDir: baseDir}
}

// PrepareWorkspace 准备执行工作空间
// 返回工作目录路径和清理函数
// 使用复制而非软链接，以支持 Docker 容器挂载
func (m *WorkspaceManager) PrepareWorkspace(taskID uuid.UUID, repoPath string) (workDir string, cleanup func(), err error) {
	workDir = filepath.Join(m.baseDir, taskID.String())

	// 创建工作目录
	if err = os.MkdirAll(workDir, 0700); err != nil {
		return "", nil, err
	}

	// 复制仓库内容到工作目录
	if err = copyDir(repoPath, workDir); err != nil {
		os.RemoveAll(workDir)
		return "", nil, err
	}

	// 返回清理函数
	cleanup = func() {
		if removeErr := os.RemoveAll(workDir); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Exec("ANSIBLE").Error("清理工作目录失败: %v", removeErr)
		}
	}

	return workDir, cleanup, nil
}

// GetWorkDir 获取任务工作目录路径
func (m *WorkspaceManager) GetWorkDir(taskID uuid.UUID) string {
	return filepath.Join(m.baseDir, taskID.String())
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算目标路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		// 跳过 .git 目录
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return copySymlink(src, path, dstPath)
		}
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("禁止复制符号链接: %s", path)
		}

		return copyFile(path, dstPath)
	})
}

func copySymlink(root, srcPath, dstPath string) error {
	target, err := os.Readlink(srcPath)
	if err != nil {
		return fmt.Errorf("读取符号链接失败: %w", err)
	}
	if filepath.IsAbs(target) {
		return fmt.Errorf("工作区不允许绝对路径符号链接: %s", srcPath)
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(srcPath), target))
	relPath, err := filepath.Rel(root, resolved)
	if err != nil {
		return fmt.Errorf("校验符号链接失败: %w", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return fmt.Errorf("工作区不允许指向仓库外的符号链接: %s -> %s", srcPath, target)
	}
	return os.Symlink(target, dstPath)
}

// copyFile 复制单个文件
func copyFile(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("工作区只允许复制普通文件: %s", src)
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}
