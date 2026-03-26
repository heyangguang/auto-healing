package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Clone 克隆仓库
func (c *Client) Clone(ctx context.Context) error {
	localPath := c.getLocalPath()
	if _, err := os.Stat(localPath); err == nil {
		if err := os.RemoveAll(localPath); err != nil {
			return fmt.Errorf("清理旧目录失败: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	url, cleanup, err := c.getAuthenticatedURL()
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	args := []string{"clone", "--branch", c.repo.DefaultBranch, "--single-branch", url, localPath}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = c.getEnv()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("克隆失败: %s", redactCredentials(stderr.String()))
	}
	return nil
}

// Pull 拉取最新代码
func (c *Client) Pull(ctx context.Context, branch string) error {
	localPath := c.getLocalPath()
	if branch == "" {
		branch = c.repo.DefaultBranch
	}

	_, cleanup, err := c.getAuthenticatedURL()
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	if err := c.checkoutBranch(ctx, localPath, branch); err != nil {
		return err
	}

	pullCmd := exec.CommandContext(ctx, "git", "pull", "origin", branch)
	pullCmd.Dir = localPath
	pullCmd.Env = c.getEnv()
	var stderr bytes.Buffer
	pullCmd.Stderr = &stderr
	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("拉取失败: %s", redactCredentials(stderr.String()))
	}
	return nil
}

func (c *Client) checkoutBranch(ctx context.Context, localPath, branch string) error {
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", branch)
	checkoutCmd.Dir = localPath
	checkoutCmd.Env = c.getEnv()
	if err := checkoutCmd.Run(); err == nil {
		return nil
	}

	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin", branch)
	fetchCmd.Dir = localPath
	fetchCmd.Env = c.getEnv()
	if err := fetchCmd.Run(); err != nil {
		return fmt.Errorf("获取分支失败: %w", err)
	}

	checkoutCmd = exec.CommandContext(ctx, "git", "checkout", "-b", branch, "origin/"+branch)
	checkoutCmd.Dir = localPath
	checkoutCmd.Env = c.getEnv()
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("切换分支失败: %w", err)
	}
	return nil
}

// ListBranches 列出远程分支
func (c *Client) ListBranches(ctx context.Context) ([]string, error) {
	localPath := c.getLocalPath()
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "--all")
	fetchCmd.Dir = localPath
	fetchCmd.Env = c.getEnv()
	fetchCmd.Run()

	cmd := exec.CommandContext(ctx, "git", "branch", "-r")
	cmd.Dir = localPath
	cmd.Env = c.getEnv()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("获取分支列表失败: %w", err)
	}
	return parseRemoteBranches(stdout.String()), nil
}

func parseRemoteBranches(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "HEAD") {
			continue
		}
		branches = append(branches, strings.TrimPrefix(line, "origin/"))
	}
	return branches
}

// ValidateAndListBranches 验证远程仓库并返回分支列表
func (c *Client) ValidateAndListBranches(ctx context.Context) (branches []string, defaultBranch string, err error) {
	authURL, cleanup, err := c.getAuthenticatedURL()
	if err != nil {
		return nil, "", fmt.Errorf("认证配置错误: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	headsOutput, err := c.runLsRemote(ctx, "--heads", authURL)
	if err != nil {
		return nil, "", err
	}
	branches = parseLsRemoteBranches(headsOutput)
	defaultBranch = c.detectDefaultBranch(ctx, authURL, branches)
	return branches, defaultBranch, nil
}

func (c *Client) runLsRemote(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"ls-remote"}, args...)...)
	cmd.Env = c.getEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", classifyLsRemoteError(stderr.String())
	}
	return stdout.String(), nil
}

func classifyLsRemoteError(stderr string) error {
	errMsg := redactCredentials(stderr)
	switch {
	case strings.Contains(errMsg, "could not read Username"),
		strings.Contains(errMsg, "Authentication failed"),
		strings.Contains(errMsg, "invalid username or password"):
		return fmt.Errorf("认证失败: 用户名或密码/令牌无效")
	case strings.Contains(errMsg, "Repository not found"), strings.Contains(errMsg, "not found"):
		return fmt.Errorf("仓库不存在或无访问权限")
	case strings.Contains(errMsg, "Could not resolve host"):
		return fmt.Errorf("无法访问仓库地址: 域名解析失败")
	case strings.Contains(errMsg, "Connection refused"), strings.Contains(errMsg, "Connection timed out"):
		return fmt.Errorf("无法连接到仓库服务器")
	default:
		return fmt.Errorf("仓库验证失败: %s", errMsg)
	}
}

func parseLsRemoteBranches(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if parts := strings.Split(line, "refs/heads/"); len(parts) == 2 {
			branches = append(branches, parts[1])
		}
	}
	return branches
}

func (c *Client) detectDefaultBranch(ctx context.Context, authURL string, branches []string) string {
	output, err := c.runLsRemote(ctx, "--symref", authURL, "HEAD")
	if err == nil {
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "ref: refs/heads/") {
				if parts := strings.Split(line, "refs/heads/"); len(parts) == 2 {
					return strings.Fields(parts[1])[0]
				}
			}
		}
	}
	if len(branches) > 0 {
		return branches[0]
	}
	return ""
}

// ValidateRemote 验证远程仓库（兼容旧代码）
func (c *Client) ValidateRemote(ctx context.Context) error {
	branches, _, err := c.ValidateAndListBranches(ctx)
	if err != nil {
		return err
	}
	if c.repo.DefaultBranch == "" {
		return nil
	}
	for _, branch := range branches {
		if branch == c.repo.DefaultBranch {
			return nil
		}
	}
	return fmt.Errorf("分支 '%s' 不存在，可用分支: %s", c.repo.DefaultBranch, strings.Join(branches, ", "))
}
