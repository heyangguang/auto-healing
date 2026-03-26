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
		mustUnmarshalAuthConfig(c.repo.AuthConfig, &config)
		if strings.HasPrefix(url, "https://") {
			url = strings.Replace(url, "https://", "https://"+config.Token+"@", 1)
		}
		return url, nil, nil
	case "password":
		config := model.PasswordAuthConfig{}
		mustUnmarshalAuthConfig(c.repo.AuthConfig, &config)
		if strings.HasPrefix(url, "https://") {
			url = strings.Replace(url, "https://", fmt.Sprintf("https://%s:%s@", config.Username, config.Password), 1)
		} else if strings.HasPrefix(url, "http://") {
			url = strings.Replace(url, "http://", fmt.Sprintf("http://%s:%s@", config.Username, config.Password), 1)
		}
		return url, nil, nil
	case "ssh_key":
		return c.buildSSHKeyAuth(url)
	default:
		return url, nil, nil
	}
}

func mustUnmarshalAuthConfig(raw any, target any) {
	configBytes, _ := json.Marshal(raw)
	_ = json.Unmarshal(configBytes, target)
}

func (c *Client) buildSSHKeyAuth(url string) (string, func(), error) {
	config := model.SSHKeyAuthConfig{}
	mustUnmarshalAuthConfig(c.repo.AuthConfig, &config)

	tmpFile, err := os.CreateTemp("", "git-ssh-key-*")
	if err != nil {
		return "", nil, err
	}
	privateKey := normalizePrivateKey(config.PrivateKey)
	if _, err := tmpFile.WriteString(privateKey); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", nil, err
	}
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0600)

	sshCmd, err := buildGitSSHCommand(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", nil, err
	}
	c.extraEnv = append(c.extraEnv, "GIT_SSH_COMMAND="+sshCmd)
	return url, func() { os.Remove(tmpFile.Name()) }, nil
}

func normalizePrivateKey(privateKey string) string {
	privateKey = strings.ReplaceAll(privateKey, "\\n", "\n")
	if !strings.HasSuffix(privateKey, "\n") {
		privateKey += "\n"
	}
	return privateKey
}
