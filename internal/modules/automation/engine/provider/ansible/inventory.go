package ansible

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const (
	inventoryBuildErrorPrefix = "__AUTO_HEALING_INVENTORY_ERROR__:"
	invalidInventoryHostMarker = "__AUTO_HEALING_INVALID_HOST__"
)

// HostCredential 主机认证信息
type HostCredential struct {
	Host       string // 主机地址
	AuthType   string // ssh_key 或 password
	Username   string // SSH 用户名
	PrivateKey string // 私钥内容（ssh_key 方式）
	Password   string // 密码（password 方式）
	KeyFile    string // 临时密钥文件路径（由执行器设置）
}

// GenerateInventory 从主机列表生成 INI 格式 inventory
// hosts 格式: "host1,host2" 或 "host1:port,host2:port"
func GenerateInventory(hosts string, groupName string, vars map[string]string) string {
	content, err := buildInventory(hosts, groupName, vars)
	if err != nil {
		return inventoryBuildErrorPrefix + err.Error()
	}
	return content
}

func buildInventory(hosts string, groupName string, vars map[string]string) (string, error) {
	if groupName == "" {
		groupName = "targets"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s]\n", groupName))

	for _, host := range strings.Split(hosts, ",") {
		appendInventoryHostLine(&sb, host)
	}

	if len(vars) > 0 {
		sb.WriteString(fmt.Sprintf("\n[%s:vars]\n", groupName))
		for key, value := range vars {
			safeKey, err := validateInventoryToken("var_key", key)
			if err != nil {
				return "", err
			}
			sb.WriteString(fmt.Sprintf("%s=%s\n", safeKey, quoteInventoryValue(strings.TrimSpace(value))))
		}
	}

	return sb.String(), nil
}

// GenerateInventoryWithAuth 生成带认证参数的 inventory
// 每台主机可以有不同的认证方式
func GenerateInventoryWithAuth(credentials []HostCredential, groupName string) string {
	content, err := buildInventoryWithAuth(credentials, groupName)
	if err != nil {
		return inventoryBuildErrorPrefix + err.Error()
	}
	return content
}

func buildInventoryWithAuth(credentials []HostCredential, groupName string) (string, error) {
	if groupName == "" {
		groupName = "targets"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s]\n", groupName))

	for _, cred := range credentials {
		host := strings.TrimSpace(cred.Host)
		if !isValidInventoryHost(host) {
			writeInvalidInventoryHost(&sb, host)
			continue
		}

		line := host
		if cred.Username != "" {
			line += fmt.Sprintf(" ansible_user=%s", quoteInventoryValue(strings.TrimSpace(cred.Username)))
		}

		authPart, err := inventoryAuthPart(cred)
		if err != nil {
			return "", err
		}
		line += authPart
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("\n[%s:vars]\n", groupName))
	sb.WriteString("ansible_python_interpreter=auto\n")
	return sb.String(), nil
}

func inventoryAuthPart(cred HostCredential) (string, error) {
	switch cred.AuthType {
	case "", "none":
		return "", nil
	case "ssh_key":
		if strings.TrimSpace(cred.KeyFile) == "" {
			return "", fmt.Errorf("ssh_key 认证缺少 key_file")
		}
		return fmt.Sprintf(" ansible_ssh_private_key_file=%s", quoteInventoryValue(strings.TrimSpace(cred.KeyFile))), nil
	case "password":
		if cred.Password == "" {
			return "", fmt.Errorf("password 认证缺少 password")
		}
		return fmt.Sprintf(" ansible_ssh_pass=%s", quoteInventoryValue(cred.Password)), nil
	default:
		return "", fmt.Errorf("不支持的 auth_type: %s", cred.AuthType)
	}
}

func validateInventoryToken(field, value string) (string, error) {
	token := strings.TrimSpace(value)
	if token == "" {
		return "", fmt.Errorf("%s 不能为空", field)
	}
	if hasUnsafeInventoryRunes(token) {
		return "", fmt.Errorf("%s 包含不安全字符", field)
	}
	return token, nil
}

func hasUnsafeInventoryRunes(value string) bool {
	for _, r := range value {
		if r <= 32 {
			return true
		}
	}
	return strings.Contains(value, "=")
}

// WriteKeyFile 写入临时密钥文件
// 返回文件路径
func WriteKeyFile(workDir, keyName, content string) (string, error) {
	safeName, err := validateKeyFileName(keyName)
	if err != nil {
		return "", err
	}
	path := filepath.Join(workDir, safeName)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("写入密钥文件失败: %w", err)
	}
	return path, nil
}

func validateKeyFileName(keyName string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(keyName))
	if clean == "" || clean == "." {
		return "", fmt.Errorf("无效的密钥文件名")
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("密钥文件名不能是绝对路径: %s", keyName)
	}
	if filepath.Base(clean) != clean {
		return "", fmt.Errorf("密钥文件名不能包含路径: %s", keyName)
	}
	return clean, nil
}

// WriteInventoryFile 写入临时 inventory 文件
func WriteInventoryFile(workDir, content string) (path string, err error) {
	if buildErr, ok := extractInventoryBuildError(content); ok {
		return "", fmt.Errorf("生成 inventory 失败: %s", buildErr)
	}
	if err = validateGeneratedInventory(content); err != nil {
		return "", err
	}
	path = filepath.Join(workDir, "inventory.ini")
	err = os.WriteFile(path, []byte(content), 0600)
	return path, err
}

func extractInventoryBuildError(content string) (string, bool) {
	if !strings.HasPrefix(content, inventoryBuildErrorPrefix) {
		return "", false
	}
	return strings.TrimPrefix(content, inventoryBuildErrorPrefix), true
}

// GenerateAnsibleCfg 生成 ansible.cfg 配置
func GenerateAnsibleCfg(options map[string]string) string {
	var sb strings.Builder
	sb.WriteString("[defaults]\n")
	sb.WriteString("host_key_checking = True\n")
	sb.WriteString("retry_files_enabled = False\n")
	sb.WriteString("gathering = smart\n")

	for k, v := range options {
		sb.WriteString(fmt.Sprintf("%s = %s\n", k, v))
	}

	return sb.String()
}

// WriteAnsibleCfg 写入 ansible.cfg 文件
func WriteAnsibleCfg(workDir string, options map[string]string) error {
	content := GenerateAnsibleCfg(options)
	path := filepath.Join(workDir, "ansible.cfg")
	return os.WriteFile(path, []byte(content), 0600)
}

func quoteInventoryValue(value string) string {
	return strconv.Quote(strings.ReplaceAll(value, "\r", ""))
}

func appendInventoryHostLine(sb *strings.Builder, host string) {
	host = strings.TrimSpace(host)
	if host == "" {
		return
	}
	if !isValidInventoryHost(host) {
		writeInvalidInventoryHost(sb, host)
		return
	}
	sb.WriteString(host)
	sb.WriteString("\n")
}

func isValidInventoryHost(host string) bool {
	if host == "" || strings.ContainsAny(host, "#=") {
		return false
	}
	for _, r := range host {
		if unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func writeInvalidInventoryHost(sb *strings.Builder, host string) {
	sb.WriteString(invalidInventoryHostMarker)
	sb.WriteString(" ")
	sb.WriteString(strconv.Quote(host))
	sb.WriteString("\n")
}

func validateGeneratedInventory(content string) error {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, invalidInventoryHostMarker+" ") {
			host, err := strconv.Unquote(strings.TrimPrefix(line, invalidInventoryHostMarker+" "))
			if err != nil {
				return fmt.Errorf("inventory 主机格式非法")
			}
			return fmt.Errorf("inventory 主机格式非法: %s", host)
		}
	}
	return nil
}
