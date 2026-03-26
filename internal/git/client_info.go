package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GetCommits 获取最近的 commit 历史
func (c *Client) GetCommits(ctx context.Context, limit int) ([]CommitInfo, error) {
	limit = normalizeCommitLimit(limit)
	format := "%H|%h|%s|%an|%ae|%aI"
	cmd := exec.CommandContext(ctx, "git", "log", fmt.Sprintf("-n%d", limit), fmt.Sprintf("--format=%s", format))
	cmd.Dir = c.getLocalPath()
	cmd.Env = c.getEnv()

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("获取 commit 历史失败: %w", err)
	}
	return parseCommitLog(stdout.String()), nil
}

func normalizeCommitLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func parseCommitLog(output string) []CommitInfo {
	var commits []CommitInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if parts := strings.SplitN(line, "|", 6); len(parts) == 6 {
			commits = append(commits, CommitInfo{
				FullID:      parts[0],
				CommitID:    parts[1],
				Message:     parts[2],
				Author:      parts[3],
				AuthorEmail: parts[4],
				Date:        parts[5],
			})
		}
	}
	return commits
}

// GetLatestCommitID 获取最新的 commit ID（短哈希）
func (c *Client) GetLatestCommitID(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	cmd.Dir = c.getLocalPath()
	cmd.Env = c.getEnv()

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("获取 commit ID 失败: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}
