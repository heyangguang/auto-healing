package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// GetFiles 获取文件树
func (s *Service) GetFiles(ctx context.Context, id uuid.UUID) ([]FileInfo, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if repo.LocalPath == "" || repo.Status != "ready" {
		return nil, fmt.Errorf("仓库未同步")
	}
	return s.scanDirectory(repo.LocalPath, "")
}

func (s *Service) scanDirectory(basePath, relativePath string) ([]FileInfo, error) {
	entries, err := os.ReadDir(filepath.Join(basePath, relativePath))
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		info, err := s.buildFileInfo(basePath, relativePath, entry)
		if err != nil {
			return nil, err
		}
		files = append(files, info)
	}
	return files, nil
}

func (s *Service) buildFileInfo(basePath, relativePath string, entry os.DirEntry) (FileInfo, error) {
	info := FileInfo{
		Name: entry.Name(),
		Path: filepath.Join(relativePath, entry.Name()),
	}
	if entry.IsDir() {
		info.Type = "directory"
		children, err := s.scanDirectory(basePath, info.Path)
		if err != nil {
			return FileInfo{}, err
		}
		info.Children = children
		return info, nil
	}

	info.Type = "file"
	if stat, err := entry.Info(); err == nil {
		info.Size = stat.Size()
	}
	return info, nil
}

// GetFileContent 获取文件内容
func (s *Service) GetFileContent(ctx context.Context, id uuid.UUID, path string) (string, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if repo.LocalPath == "" {
		return "", fmt.Errorf("仓库未同步")
	}

	fullPath, err := resolveRepoFilePath(repo.LocalPath, path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func resolveRepoFilePath(repoRoot, relPath string) (string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(filepath.Join(resolvedRoot, relPath))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("非法路径")
	}
	return resolvedPath, nil
}

// ScanVariables 扫描 Playbook 变量
func (s *Service) ScanVariables(ctx context.Context, id uuid.UUID, mainPlaybook string) ([]PlaybookVariable, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if repo.LocalPath == "" {
		return nil, fmt.Errorf("仓库未同步")
	}

	content, err := os.ReadFile(filepath.Join(repo.LocalPath, mainPlaybook))
	if err != nil {
		return nil, fmt.Errorf("无法读取 playbook: %v", err)
	}
	variables := s.extractVariables(string(content))
	logger.Sync_("GIT").Info("仓库: %s | 入口: %s | 发现变量: %d 个", repo.Name, mainPlaybook, len(variables))
	return variables, nil
}

func (s *Service) extractVariables(content string) []PlaybookVariable {
	varMap := make(map[string]bool)
	variables := make([]PlaybookVariable, 0)
	inVar := false
	varStart := 0

	for i := 0; i < len(content)-1; i++ {
		switch {
		case content[i] == '{' && content[i+1] == '{':
			inVar = true
			varStart = i + 2
		case inVar && content[i] == '}' && content[i+1] == '}':
			name := s.cleanVarName(content[varStart:i])
			if name != "" && !varMap[name] {
				varMap[name] = true
				variables = append(variables, PlaybookVariable{Name: name, Type: "string"})
			}
			inVar = false
		}
	}
	return variables
}

func (s *Service) cleanVarName(raw string) string {
	raw = strings.TrimSpace(raw)
	var result []byte
	for _, c := range raw {
		if c == ' ' || c == '\t' || c == '\n' {
			continue
		}
		if c == '|' {
			break
		}
		result = append(result, byte(c))
	}
	return string(result)
}
