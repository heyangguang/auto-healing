package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
)

// 允许的路径前缀（安全白名单）- 只允许专用目录
var allowedPathPrefixes = []string{
	"/etc/auto-healing/secrets/",
}

// 禁止的文件名（安全黑名单）
var forbiddenFiles = []string{
	"passwd",
	"shadow",
	"sudoers",
	"hosts",
}

// FileProvider 文件密钥提供者（只支持 ssh_key）
type FileProvider struct {
	config secretsmodel.FileConfig
	name   string
}

// NewFileProvider 创建文件密钥提供者
func NewFileProvider(source *secretsmodel.SecretsSource) (*FileProvider, error) {
	if err := validateSecretAuthType(source.AuthType); err != nil {
		return nil, err
	}
	if source.AuthType != "ssh_key" {
		return nil, newConfigError("file 密钥源只支持 ssh_key 认证")
	}

	var config secretsmodel.FileConfig
	configBytes, _ := json.Marshal(source.Config)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, newConfigError(fmt.Sprintf("解析文件配置失败: %v", err))
	}

	if config.KeyPath == "" {
		return nil, newConfigError("key_path 不能为空")
	}

	// 安全验证：路径规范化
	cleanPath := filepath.Clean(config.KeyPath)

	// 安全验证：禁止路径遍历
	if strings.Contains(cleanPath, "..") {
		return nil, newConfigError("路径不能包含 '..'")
	}

	// 安全验证：检查路径白名单
	if err := validateAllowedPath(cleanPath); err != nil {
		return nil, err
	}

	// 安全验证：检查文件名黑名单
	fileName := filepath.Base(cleanPath)
	for _, forbidden := range forbiddenFiles {
		if fileName == forbidden {
			return nil, newConfigError(fmt.Sprintf("不允许访问敏感文件: %s", forbidden))
		}
	}
	if err := validateExistingKeyPath(cleanPath); err != nil {
		return nil, err
	}

	config.KeyPath = cleanPath
	if config.Username == "" {
		config.Username = "root"
	}

	return &FileProvider{
		config: config,
		name:   source.Name,
	}, nil
}

// GetSecret 获取密钥（file 类型只返回 ssh_key）
func (p *FileProvider) GetSecret(ctx context.Context, query secretsmodel.SecretQuery) (*secretsmodel.Secret, error) {
	resolvedPath, err := p.resolveKeyPath()
	if err != nil {
		return nil, err
	}

	// 检查文件是否存在
	if _, err := os.Stat(resolvedPath); err != nil {
		if os.IsNotExist(err) {
			return nil, newConnectionError("密钥文件不存在", err)
		}
		return nil, newConnectionError("检查密钥文件失败", err)
	}

	// 读取文件内容
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, newConnectionError("读取密钥文件失败", err)
	}

	return &secretsmodel.Secret{
		AuthType:   "ssh_key",
		Username:   p.config.Username,
		PrivateKey: string(content),
	}, nil
}

// TestConnection 测试连接
func (p *FileProvider) TestConnection(ctx context.Context) error {
	resolvedPath, err := p.resolveKeyPath()
	if err != nil {
		return err
	}

	// 检查文件是否存在
	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return newConnectionError("密钥文件不存在", err)
		}
		return newConnectionError("检查密钥文件失败", err)
	}
	if info.IsDir() {
		return newConfigError("key_path 必须指向文件，不能是目录")
	}
	return nil
}

// Name 获取提供者名称
func (p *FileProvider) Name() string {
	return p.name
}

func (p *FileProvider) resolveKeyPath() (string, error) {
	resolvedPath, err := filepath.EvalSymlinks(p.config.KeyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return p.config.KeyPath, nil
		}
		return "", newConnectionError("解析密钥文件路径失败", err)
	}
	resolvedPath = filepath.Clean(resolvedPath)
	if err := validateAllowedPath(resolvedPath); err != nil {
		return "", newConfigError("密钥文件解析后超出允许目录")
	}
	return resolvedPath, nil
}

func validateExistingKeyPath(keyPath string) error {
	resolvedPath, err := filepath.EvalSymlinks(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return newConnectionError("解析密钥文件路径失败", err)
	}
	resolvedPath = filepath.Clean(resolvedPath)
	if err := validateAllowedPath(resolvedPath); err != nil {
		return newConfigError("密钥文件解析后超出允许目录")
	}
	fileInfo, err := os.Stat(resolvedPath)
	if err != nil {
		return newConnectionError("检查密钥文件失败", err)
	}
	if fileInfo.IsDir() {
		return newConfigError("key_path 必须指向文件，不能是目录")
	}
	return nil
}

func validateAllowedPath(path string) error {
	if isAllowedPath(path) {
		return nil
	}
	return newConfigError(fmt.Sprintf("路径不在允许的目录中，允许的目录: %v", allowedPathPrefixes))
}

func isAllowedPath(path string) bool {
	cleanPath := filepath.Clean(path)
	for _, prefix := range allowedPathPrefixes {
		cleanPrefix := filepath.Clean(prefix)
		if cleanPath == cleanPrefix || strings.HasPrefix(cleanPath, cleanPrefix+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}
