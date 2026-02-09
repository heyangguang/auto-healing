package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/company/auto-healing/internal/model"
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
	config model.FileConfig
	name   string
}

// NewFileProvider 创建文件密钥提供者
func NewFileProvider(source *model.SecretsSource) (*FileProvider, error) {
	var config model.FileConfig
	configBytes, _ := json.Marshal(source.Config)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("解析文件配置失败: %w", err)
	}

	if config.KeyPath == "" {
		return nil, fmt.Errorf("key_path 不能为空")
	}

	// 安全验证：路径规范化
	cleanPath := filepath.Clean(config.KeyPath)

	// 安全验证：禁止路径遍历
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("路径不能包含 '..'")
	}

	// 安全验证：检查路径白名单
	allowed := false
	for _, prefix := range allowedPathPrefixes {
		if strings.HasPrefix(cleanPath, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("路径不在允许的目录中，允许的目录: %v", allowedPathPrefixes)
	}

	// 安全验证：检查文件名黑名单
	fileName := filepath.Base(cleanPath)
	for _, forbidden := range forbiddenFiles {
		if fileName == forbidden {
			return nil, fmt.Errorf("不允许访问敏感文件: %s", forbidden)
		}
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
func (p *FileProvider) GetSecret(ctx context.Context, query model.SecretQuery) (*model.Secret, error) {
	// 检查文件是否存在
	if _, err := os.Stat(p.config.KeyPath); os.IsNotExist(err) {
		return nil, ErrSecretNotFound
	}

	// 读取文件内容
	content, err := os.ReadFile(p.config.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("读取密钥文件失败: %w", err)
	}

	return &model.Secret{
		AuthType:   "ssh_key",
		Username:   p.config.Username,
		PrivateKey: string(content),
	}, nil
}

// TestConnection 测试连接
func (p *FileProvider) TestConnection(ctx context.Context) error {
	// 检查文件是否存在
	if _, err := os.Stat(p.config.KeyPath); os.IsNotExist(err) {
		return fmt.Errorf("密钥文件不存在: %s", p.config.KeyPath)
	}
	return nil
}

// Name 获取提供者名称
func (p *FileProvider) Name() string {
	return p.name
}
