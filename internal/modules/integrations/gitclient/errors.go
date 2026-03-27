package git

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const (
	ErrorCodeKnownHostsRequired = "SSH_KNOWN_HOSTS_REQUIRED"
	knownHostsEnvVar            = "AUTO_HEALING_KNOWN_HOSTS"
)

var authenticatedURLPattern = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^/\s:@]+(?::[^/\s@]*)?@)`)

type KnownHostsRequiredError struct {
	DefaultPath string
}

func (e *KnownHostsRequiredError) Error() string {
	return fmt.Sprintf(
		"SSH 仓库缺少受信任主机配置：请设置 %s 或准备 %s 后重试",
		knownHostsEnvVar,
		e.DefaultPath,
	)
}

func (e *KnownHostsRequiredError) ErrorCode() string {
	return ErrorCodeKnownHostsRequired
}

func (e *KnownHostsRequiredError) ErrorDetails() any {
	return map[string]string{
		"env_var":      knownHostsEnvVar,
		"default_path": e.DefaultPath,
	}
}

func newKnownHostsRequiredError() error {
	return &KnownHostsRequiredError{DefaultPath: defaultKnownHostsPath()}
}

func defaultKnownHostsPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return "~/.ssh/known_hosts"
	}
	return filepath.Join(homeDir, ".ssh", "known_hosts")
}

func redactCredentials(message string) string {
	return authenticatedURLPattern.ReplaceAllString(message, "${1}***@")
}
