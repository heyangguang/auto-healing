package git

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/company/auto-healing/internal/model"
)

// getAuthenticatedURL 获取带认证的 URL，同时设置 c.extraEnv
func (c *Client) getAuthenticatedURL() (string, func(), error) {
	url := c.repo.URL
	c.extraEnv = nil

	switch c.repo.AuthType {
	case "token":
		config := model.TokenAuthConfig{}
		if err := unmarshalAuthConfig(c.repo.AuthConfig, &config); err != nil {
			return "", nil, err
		}
		if strings.TrimSpace(config.Token) == "" {
			return "", nil, fmt.Errorf("token 认证缺少 token")
		}
		if strings.HasPrefix(url, "https://") {
			url = strings.Replace(url, "https://", "https://"+config.Token+"@", 1)
		} else if strings.HasPrefix(url, "http://") {
			url = strings.Replace(url, "http://", "http://"+config.Token+"@", 1)
		} else {
			return "", nil, fmt.Errorf("token 认证不支持当前仓库地址协议: %s", url)
		}
		return url, nil, nil
	case "password":
		config := model.PasswordAuthConfig{}
		if err := unmarshalAuthConfig(c.repo.AuthConfig, &config); err != nil {
			return "", nil, err
		}
		if strings.TrimSpace(config.Username) == "" || config.Password == "" {
			return "", nil, fmt.Errorf("password 认证缺少用户名或密码")
		}
		if strings.HasPrefix(url, "https://") {
			url = strings.Replace(url, "https://", fmt.Sprintf("https://%s:%s@", config.Username, config.Password), 1)
		} else if strings.HasPrefix(url, "http://") {
			url = strings.Replace(url, "http://", fmt.Sprintf("http://%s:%s@", config.Username, config.Password), 1)
		} else {
			return "", nil, fmt.Errorf("password 认证不支持当前仓库地址协议: %s", url)
		}
		return url, nil, nil
	case "ssh_key":
		return c.buildSSHKeyAuth(url)
	default:
		return url, nil, nil
	}
}

func unmarshalAuthConfig(raw any, target any) error {
	configBytes, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("序列化认证配置失败: %w", err)
	}
	if err := json.Unmarshal(configBytes, target); err != nil {
		return fmt.Errorf("解析认证配置失败: %w", err)
	}
	return nil
}

func (c *Client) buildSSHKeyAuth(url string) (string, func(), error) {
	config := model.SSHKeyAuthConfig{}
	if err := unmarshalAuthConfig(c.repo.AuthConfig, &config); err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(config.PrivateKey) == "" {
		return "", nil, fmt.Errorf("ssh_key 认证缺少 private_key")
	}

	tmpFile, err := os.CreateTemp("", "git-ssh-key-*")
	if err != nil {
		return "", nil, err
	}
	privateKey := normalizePrivateKey(config.PrivateKey)
	cleanup := func() { _ = os.Remove(tmpFile.Name()) }
	if err := writeSSHPrivateKeyFile(tmpFile, privateKey); err != nil {
		cleanup()
		return "", nil, err
	}

	sshCmd, err := buildGitSSHCommand(tmpFile.Name())
	if err != nil {
		cleanup()
		return "", nil, err
	}
	c.extraEnv = append(c.extraEnv, "GIT_SSH_COMMAND="+sshCmd)
	return url, cleanup, nil
}

func writeSSHPrivateKeyFile(tmpFile *os.File, privateKey string) error {
	if _, err := tmpFile.WriteString(privateKey); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("关闭 SSH 私钥临时文件失败: %w", err)
	}
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		return fmt.Errorf("设置 SSH 私钥文件权限失败: %w", err)
	}
	return nil
}

func normalizePrivateKey(privateKey string) string {
	privateKey = strings.ReplaceAll(privateKey, "\\n", "\n")
	if !strings.HasSuffix(privateKey, "\n") {
		privateKey += "\n"
	}
	return privateKey
}
