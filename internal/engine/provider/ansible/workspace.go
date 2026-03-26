package ansible

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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
		os.RemoveAll(workDir)
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

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("禁止复制符号链接: %s", path)
		}

		return copyFile(path, dstPath)
	})
}

// copyFile 复制单个文件
func copyFile(src, dst string) error {
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

	// 复制权限
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}
