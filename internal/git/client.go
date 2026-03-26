package git

import (
	"fmt"
	"os"
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

// CommitInfo Commit 信息
type CommitInfo struct {
	CommitID    string `json:"commit_id"`
	FullID      string `json:"full_id"`
	Message     string `json:"message"`
	Author      string `json:"author"`
	AuthorEmail string `json:"author_email"`
	Date        string `json:"date"`
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

// getEnv 获取环境变量
func (c *Client) getEnv() []string {
	env := os.Environ()
	// 禁用交互式提示
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	// 添加额外环境变量（如 SSH 认证）
	env = append(env, c.extraEnv...)
	return env
}

func buildGitSSHCommand(keyPath string) (string, error) {
	if knownHostsPath := gitKnownHostsPath(); knownHostsPath != "" {
		return fmt.Sprintf(
			"ssh -i %s -o StrictHostKeyChecking=yes -o UserKnownHostsFile=%s",
			shellQuoteSSHArg(keyPath),
			shellQuoteSSHArg(knownHostsPath),
		), nil
	}
	return "", newKnownHostsRequiredError()
}

func shellQuoteSSHArg(arg string) string {
	if arg == "" {
		return "''"
	}
	quote := "'"
	return quote + strings.ReplaceAll(arg, quote, `'"\''"`) + quote
}

func gitKnownHostsPath() string {
	if path := strings.TrimSpace(os.Getenv("AUTO_HEALING_KNOWN_HOSTS")); path != "" {
		return path
	}

	path := defaultKnownHostsPath()
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}
