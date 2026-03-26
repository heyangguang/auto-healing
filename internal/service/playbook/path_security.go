package playbook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errIllegalRepoPath = errors.New("非法路径")

func normalizeRepoRelativePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errIllegalRepoPath
	}
	if filepath.IsAbs(trimmed) {
		return "", errIllegalRepoPath
	}

	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", errIllegalRepoPath
	}
	return cleaned, nil
}

func resolveExistingRepoPath(repoRoot, path string) (string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", err
	}

	candidate, err := repoCandidatePath(resolvedRoot, path)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	if !pathWithinRepo(resolvedRoot, resolvedPath) {
		return "", errIllegalRepoPath
	}
	return resolvedPath, nil
}

func repoPathExists(repoRoot, path string) (bool, error) {
	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return false, err
	}
	candidate, err := repoCandidatePath(resolvedRoot, path)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(candidate); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if _, err := resolveExistingRepoPath(resolvedRoot, candidate); err != nil {
		return false, err
	}
	return true, nil
}

func repoCandidatePath(repoRoot, path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errIllegalRepoPath
	}
	if filepath.IsAbs(trimmed) {
		cleaned := filepath.Clean(trimmed)
		if !pathWithinRepo(repoRoot, cleaned) {
			return "", errIllegalRepoPath
		}
		return cleaned, nil
	}

	cleaned, err := normalizeRepoRelativePath(trimmed)
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, cleaned), nil
}

func pathWithinRepo(repoRoot, target string) bool {
	relative, err := filepath.Rel(repoRoot, target)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func invalidRepoPathError(path string) error {
	return fmt.Errorf("非法路径: %s", path)
}
