package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/company/auto-healing/internal/model"
)

// Client Git 客户端封装
type Client struct {
	repo     *model.GitRepository
	reposDir string   // 仓库存储基础目录
	extraEnv []string // 额外环境变量（如 SSH 认证）
}

// NewClient 创建 Git 客户端
func NewClient(repo *model.GitRepository, reposDir string) *Client {
	return &Client{
		repo:     repo,
		reposDir: reposDir,
	}
}

// Clone 克隆仓库
func (c *Client) Clone(ctx context.Context) error {
	localPath := c.getLocalPath()

	// 如果目录已存在，先删除
	if _, err := os.Stat(localPath); err == nil {
		if err := os.RemoveAll(localPath); err != nil {
			return fmt.Errorf("清理旧目录失败: %w", err)
		}
	}

	// 创建父目录
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 构建 clone 命令
	args := []string{"clone", "--branch", c.repo.DefaultBranch, "--single-branch"}

	// 设置认证
	url, cleanup, err := c.getAuthenticatedURL()
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	args = append(args, url, localPath)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = c.getEnv()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("克隆失败: %s", stderr.String())
	}

	return nil
}

// Pull 拉取最新代码
func (c *Client) Pull(ctx context.Context, branch string) error {
	localPath := c.getLocalPath()

	if branch == "" {
		branch = c.repo.DefaultBranch
	}

	// 设置认证（用于 fetch/pull）
	_, cleanup, err := c.getAuthenticatedURL()
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// 先 checkout 到指定分支
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", branch)
	checkoutCmd.Dir = localPath
	checkoutCmd.Env = c.getEnv()
	if err := checkoutCmd.Run(); err != nil {
		// 如果分支不存在，尝试 fetch
		fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin", branch)
		fetchCmd.Dir = localPath
		fetchCmd.Env = c.getEnv()
		if err := fetchCmd.Run(); err != nil {
			return fmt.Errorf("获取分支失败: %w", err)
		}
		// 再次 checkout
		checkoutCmd = exec.CommandContext(ctx, "git", "checkout", "-b", branch, "origin/"+branch)
		checkoutCmd.Dir = localPath
		checkoutCmd.Env = c.getEnv()
		if err := checkoutCmd.Run(); err != nil {
			return fmt.Errorf("切换分支失败: %w", err)
		}
	}

	// pull 更新
	pullCmd := exec.CommandContext(ctx, "git", "pull", "origin", branch)
	pullCmd.Dir = localPath
	pullCmd.Env = c.getEnv()

	var stderr bytes.Buffer
	pullCmd.Stderr = &stderr

	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("拉取失败: %s", stderr.String())
	}

	return nil
}

// ListBranches 列出远程分支
func (c *Client) ListBranches(ctx context.Context) ([]string, error) {
	localPath := c.getLocalPath()

	// 先 fetch
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "--all")
	fetchCmd.Dir = localPath
	fetchCmd.Env = c.getEnv()
	fetchCmd.Run() // 忽略错误

	// 获取远程分支
	cmd := exec.CommandContext(ctx, "git", "branch", "-r")
	cmd.Dir = localPath
	cmd.Env = c.getEnv()

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("获取分支列表失败: %w", err)
	}

	var branches []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "HEAD") {
			continue
		}
		// 去掉 origin/ 前缀
		branch := strings.TrimPrefix(line, "origin/")
		branches = append(branches, branch)
	}

	return branches, nil
}

// ValidateAndListBranches 验证远程仓库并返回分支列表（不需要本地克隆）
// 检查：URL 是否可访问、认证是否正确
// 返回：分支列表、默认分支
func (c *Client) ValidateAndListBranches(ctx context.Context) (branches []string, defaultBranch string, err error) {
	// 获取认证 URL
	authURL, cleanup, authErr := c.getAuthenticatedURL()
	if authErr != nil {
		err = fmt.Errorf("认证配置错误: %w", authErr)
		return
	}
	if cleanup != nil {
		defer cleanup()
	}

	// 使用 git ls-remote 检查远程仓库是否可访问
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", authURL)
	cmd.Env = c.getEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if cmdErr := cmd.Run(); cmdErr != nil {
		errMsg := stderr.String()
		// 分析错误类型
		if strings.Contains(errMsg, "could not read Username") ||
			strings.Contains(errMsg, "Authentication failed") ||
			strings.Contains(errMsg, "invalid username or password") {
			err = fmt.Errorf("认证失败: 用户名或密码/令牌无效")
			return
		}
		if strings.Contains(errMsg, "Repository not found") ||
			strings.Contains(errMsg, "not found") {
			err = fmt.Errorf("仓库不存在或无访问权限")
			return
		}
		if strings.Contains(errMsg, "Could not resolve host") {
			err = fmt.Errorf("无法访问仓库地址: 域名解析失败")
			return
		}
		if strings.Contains(errMsg, "Connection refused") ||
			strings.Contains(errMsg, "Connection timed out") {
			err = fmt.Errorf("无法连接到仓库服务器")
			return
		}
		err = fmt.Errorf("仓库验证失败: %s", errMsg)
		return
	}

	// 解析分支列表
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "refs/heads/")
		if len(parts) == 2 {
			branches = append(branches, parts[1])
		}
	}

	// 尝试获取默认分支（HEAD 指向）
	headCmd := exec.CommandContext(ctx, "git", "ls-remote", "--symref", authURL, "HEAD")
	headCmd.Env = c.getEnv()
	var headOut bytes.Buffer
	headCmd.Stdout = &headOut
	if headCmd.Run() == nil {
		// 解析：ref: refs/heads/main	HEAD
		for _, line := range strings.Split(headOut.String(), "\n") {
			if strings.Contains(line, "ref: refs/heads/") {
				parts := strings.Split(line, "refs/heads/")
				if len(parts) == 2 {
					defaultBranch = strings.Fields(parts[1])[0]
					break
				}
			}
		}
	}

	// 如果没有获取到默认分支，使用第一个分支
	if defaultBranch == "" && len(branches) > 0 {
		defaultBranch = branches[0]
	}

	return
}

// ValidateRemote 验证远程仓库（兼容旧代码）
// 检查：URL、认证、分支是否有效
func (c *Client) ValidateRemote(ctx context.Context) error {
	branches, _, err := c.ValidateAndListBranches(ctx)
	if err != nil {
		return err
	}

	// 检查指定分支是否存在
	branch := c.repo.DefaultBranch
	if branch == "" {
		return nil // 不指定分支时不验证
	}

	for _, b := range branches {
		if b == branch {
			return nil
		}
	}

	return fmt.Errorf("分支 '%s' 不存在，可用分支: %s", branch, strings.Join(branches, ", "))
}

// CommitInfo Commit 信息
type CommitInfo struct {
	CommitID    string `json:"commit_id"`
	FullID      string `json:"full_id"`
	Message     string `json:"message"`
	Author      string `json:"author"`
	AuthorEmail string `json:"author_email"`
	Date        string `json:"date"`
}

// GetCommits 获取最近的 commit 历史
func (c *Client) GetCommits(ctx context.Context, limit int) ([]CommitInfo, error) {
	localPath := c.getLocalPath()

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// git log 格式：full_id|short_id|message|author|email|date
	format := "%H|%h|%s|%an|%ae|%aI"
	cmd := exec.CommandContext(ctx, "git", "log", fmt.Sprintf("-n%d", limit), fmt.Sprintf("--format=%s", format))
	cmd.Dir = localPath
	cmd.Env = c.getEnv()

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("获取 commit 历史失败: %w", err)
	}

	var commits []CommitInfo
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) == 6 {
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

	return commits, nil
}

// GetLatestCommitID 获取最新的 commit ID（短哈希）
func (c *Client) GetLatestCommitID(ctx context.Context) (string, error) {
	localPath := c.getLocalPath()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	cmd.Dir = localPath
	cmd.Env = c.getEnv()

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("获取 commit ID 失败: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// Exists 检查本地仓库是否存在
func (c *Client) Exists() bool {
	gitDir := filepath.Join(c.getLocalPath(), ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

// getLocalPath 获取本地路径
func (c *Client) getLocalPath() string {
	if c.repo.LocalPath != "" {
		return c.repo.LocalPath
	}
	return filepath.Join(c.reposDir, c.repo.ID.String())
}

// getAuthenticatedURL 获取带认证的 URL，同时设置 c.extraEnv
func (c *Client) getAuthenticatedURL() (string, func(), error) {
	url := c.repo.URL
	c.extraEnv = nil // 重置

	switch c.repo.AuthType {
	case "token":
		var config model.TokenAuthConfig
		configBytes, _ := json.Marshal(c.repo.AuthConfig)
		json.Unmarshal(configBytes, &config)

		// https://token@github.com/xxx/yyy.git
		if strings.HasPrefix(url, "https://") {
			url = strings.Replace(url, "https://", "https://"+config.Token+"@", 1)
		}
		return url, nil, nil

	case "password":
		var config model.PasswordAuthConfig
		configBytes, _ := json.Marshal(c.repo.AuthConfig)
		json.Unmarshal(configBytes, &config)

		// https://user:pass@github.com/xxx/yyy.git 或 http://user:pass@...
		if strings.HasPrefix(url, "https://") {
			url = strings.Replace(url, "https://", fmt.Sprintf("https://%s:%s@", config.Username, config.Password), 1)
		} else if strings.HasPrefix(url, "http://") {
			url = strings.Replace(url, "http://", fmt.Sprintf("http://%s:%s@", config.Username, config.Password), 1)
		}
		return url, nil, nil

	case "ssh_key":
		var config model.SSHKeyAuthConfig
		configBytes, _ := json.Marshal(c.repo.AuthConfig)
		json.Unmarshal(configBytes, &config)

		// 写入临时密钥文件
		tmpFile, err := os.CreateTemp("", "git-ssh-key-*")
		if err != nil {
			return "", nil, err
		}
		// 确保私钥格式正确：替换转义的换行符为真实换行符，确保末尾有换行
		privateKey := strings.ReplaceAll(config.PrivateKey, "\\n", "\n")
		if !strings.HasSuffix(privateKey, "\n") {
			privateKey += "\n"
		}
		if _, err := tmpFile.WriteString(privateKey); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return "", nil, err
		}
		tmpFile.Close()
		os.Chmod(tmpFile.Name(), 0600)

		// 设置额外环境变量：GIT_SSH_COMMAND（支持自定义端口）
		// 从 URL 中提取端口（如 git@host:port:user/repo.git 或 ssh://git@host:port/user/repo.git）
		sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", tmpFile.Name())
		c.extraEnv = append(c.extraEnv, "GIT_SSH_COMMAND="+sshCmd)

		cleanup := func() {
			os.Remove(tmpFile.Name())
		}
		return url, cleanup, nil

	default:
		return url, nil, nil
	}
}

// getEnv 获取环境变量
func (c *Client) getEnv() []string {
	env := os.Environ()
	// 禁用交互式提示
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	// 添加额外环境变量（如 SSH 认证）
	env = append(env, c.extraEnv...)
	return env
}
