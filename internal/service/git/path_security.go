package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func normalizeRepoRelativePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("非法路径")
	}

	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("非法路径")
	}
	return cleaned, nil
}

func repoFileExists(repoRoot, path string) (bool, error) {
	cleaned, err := normalizeRepoRelativePath(path)
	if err != nil {
		return false, err
	}

	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return false, err
	}
	candidate := filepath.Join(resolvedRoot, cleaned)
	if _, err := os.Stat(candidate); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if _, err := resolveRepoFilePath(resolvedRoot, cleaned); err != nil {
		return false, err
	}
	return true, nil
}
