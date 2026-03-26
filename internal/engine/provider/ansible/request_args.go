package ansible

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func marshalExtraVars(extraVars map[string]any) (string, error) {
	if len(extraVars) == 0 {
		return "", nil
	}

	raw, err := json.Marshal(extraVars)
	if err != nil {
		return "", fmt.Errorf("序列化 extra_vars 失败: %w", err)
	}
	return string(raw), nil
}

func resolveInventoryFile(workDir, inventory string) (string, error) {
	if inventory == "" {
		return "", nil
	}

	if filepath.IsAbs(inventory) {
		path, err := statInventoryCandidate(inventory)
		if err != nil {
			return "", err
		}
		if path == "" {
			return "", fmt.Errorf("inventory 文件不存在: %s", inventory)
		}
		return path, nil
	}

	candidate := filepath.Clean(filepath.Join(workDir, inventory))
	if _, err := relativePathWithinWorkDir(workDir, candidate); err != nil {
		return "", err
	}
	return statInventoryCandidate(candidate)
}

func statInventoryCandidate(candidate string) (string, error) {
	info, err := os.Stat(candidate)
	if err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("inventory 路径是目录: %s", candidate)
		}
		return candidate, nil
	}
	if os.IsNotExist(err) {
		return "", nil
	}
	return "", fmt.Errorf("检查 inventory 路径失败: %w", err)
}

func resolvePathWithinWorkDir(workDir, path, label string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s 不能为空", label)
	}
	candidate := path
	if !filepath.IsAbs(path) {
		candidate = filepath.Join(workDir, path)
	}
	candidate = filepath.Clean(candidate)
	if _, err := relativePathWithinWorkDir(workDir, candidate); err != nil {
		return "", fmt.Errorf("%s 必须位于工作目录内: %s", label, path)
	}
	return candidate, nil
}

func resolveContainerPathWithinWorkDir(workDir, path, label string) (string, error) {
	resolved, err := resolvePathWithinWorkDir(workDir, path, label)
	if err != nil {
		return "", err
	}
	relPath, err := relativePathWithinWorkDir(workDir, resolved)
	if err != nil {
		return "", fmt.Errorf("%s 必须位于工作目录内: %s", label, path)
	}
	return "/workspace/" + strings.TrimPrefix(relPath, "./"), nil
}

func relativePathWithinWorkDir(workDir, path string) (string, error) {
	relPath, err := filepath.Rel(workDir, path)
	if err != nil {
		return "", fmt.Errorf("计算 inventory 相对路径失败: %w", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("inventory 文件必须位于工作目录内: %s", path)
	}
	return filepath.ToSlash(relPath), nil
}
