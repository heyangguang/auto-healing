package ansible

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if groupName == "" {
		groupName = "targets"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s]\n", groupName))

	hostList := strings.Split(hosts, ",")
	for _, host := range hostList {
		host = strings.TrimSpace(host)
		if host != "" {
			sb.WriteString(host)
			sb.WriteString("\n")
		}
	}

	// 添加组变量
	if len(vars) > 0 {
		sb.WriteString(fmt.Sprintf("\n[%s:vars]\n", groupName))
		for k, v := range vars {
			sb.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}

	return sb.String()
}

// GenerateInventoryWithAuth 生成带认证参数的 inventory
// 每台主机可以有不同的认证方式
func GenerateInventoryWithAuth(credentials []HostCredential, groupName string) string {
	if groupName == "" {
		groupName = "targets"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s]\n", groupName))

	for _, cred := range credentials {
		line := cred.Host

		// 添加 ansible_user
		if cred.Username != "" {
			line += fmt.Sprintf(" ansible_user=%s", cred.Username)
		}

		// 根据认证类型添加参数
		if cred.AuthType == "ssh_key" && cred.KeyFile != "" {
			line += fmt.Sprintf(" ansible_ssh_private_key_file=%s", cred.KeyFile)
		} else if cred.AuthType == "password" && cred.Password != "" {
			line += fmt.Sprintf(" ansible_ssh_pass=%s", cred.Password)
		}

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// 添加 group vars 设置 Python 解释器
	// 使用 auto 模式，Ansible 2.14 会自动尝试多个路径包括 /usr/libexec/platform-python
	sb.WriteString(fmt.Sprintf("\n[%s:vars]\n", groupName))
	sb.WriteString("ansible_python_interpreter=auto\n")

	return sb.String()
}

// WriteKeyFile 写入临时密钥文件
// 返回文件路径
func WriteKeyFile(workDir, keyName, content string) (string, error) {
	path := filepath.Join(workDir, keyName)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("写入密钥文件失败: %w", err)
	}
	return path, nil
}

// WriteInventoryFile 写入临时 inventory 文件
// 返回文件路径和清理函数
func WriteInventoryFile(workDir, content string) (path string, err error) {
	path = filepath.Join(workDir, "inventory.ini")
	err = os.WriteFile(path, []byte(content), 0600)
	return path, err
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
