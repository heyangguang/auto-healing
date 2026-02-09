package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
)

// WebhookProvider Webhook 密钥提供者
type WebhookProvider struct {
	config   model.WebhookConfig
	authType string // 从 SecretsSource.AuthType 获取
	name     string
	client   *http.Client
}

// NewWebhookProvider 创建 Webhook 密钥提供者
func NewWebhookProvider(source *model.SecretsSource) (*WebhookProvider, error) {
	var config model.WebhookConfig
	configBytes, _ := json.Marshal(source.Config)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("解析 Webhook 配置失败: %w", err)
	}

	if config.URL == "" {
		return nil, fmt.Errorf("Webhook URL 不能为空")
	}
	if config.Method == "" {
		return nil, fmt.Errorf("请求方法 (method) 不能为空，支持: GET, POST")
	}
	if config.QueryKey == "" {
		return nil, fmt.Errorf("查询键 (query_key) 不能为空，支持: ip, hostname")
	}
	if config.QueryKey != "ip" && config.QueryKey != "hostname" {
		return nil, fmt.Errorf("query_key 只支持 'ip' 或 'hostname'")
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	// 验证认证配置
	switch config.Auth.Type {
	case "", "none":
		config.Auth.Type = "none"
	case "basic":
		if config.Auth.Username == "" || config.Auth.Password == "" {
			return nil, fmt.Errorf("Basic 认证需要 username 和 password")
		}
	case "bearer":
		if config.Auth.Token == "" {
			return nil, fmt.Errorf("Bearer 认证需要 token")
		}
	case "api_key":
		if config.Auth.HeaderName == "" || config.Auth.APIKey == "" {
			return nil, fmt.Errorf("API Key 认证需要 header_name 和 api_key")
		}
	default:
		return nil, fmt.Errorf("不支持的认证类型: %s（支持: none, basic, bearer, api_key）", config.Auth.Type)
	}

	return &WebhookProvider{
		config:   config,
		authType: source.AuthType,
		name:     source.Name,
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}, nil
}

// applyAuth 应用认证到请求
func (p *WebhookProvider) applyAuth(req *http.Request) {
	switch p.config.Auth.Type {
	case "none":
		// 无认证
	case "basic":
		auth := base64.StdEncoding.EncodeToString(
			[]byte(p.config.Auth.Username + ":" + p.config.Auth.Password),
		)
		req.Header.Set("Authorization", "Basic "+auth)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+p.config.Auth.Token)
	case "api_key":
		req.Header.Set(p.config.Auth.HeaderName, p.config.Auth.APIKey)
	}
}

// GetSecret 获取密钥
func (p *WebhookProvider) GetSecret(ctx context.Context, query model.SecretQuery) (*model.Secret, error) {
	// 根据 query_key 拼接 URL（智能处理末尾斜杠）
	url := strings.TrimSuffix(p.config.URL, "/")
	switch p.config.QueryKey {
	case "ip":
		url = url + "/" + query.IPAddress
	case "hostname":
		url = url + "/" + query.Hostname
		// 如果没有配置 query_key，使用原始 URL（所有主机共用）
	}

	req, err := http.NewRequestWithContext(ctx, p.config.Method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 应用认证
	p.applyAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Webhook 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrSecretNotFound
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Webhook 返回错误: HTTP %d, %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 Webhook 响应失败: %w", err)
	}

	// 根据配置的 response_data_path 提取数据根节点
	data := result
	if p.config.ResponseDataPath != "" {
		parts := strings.Split(p.config.ResponseDataPath, ".")
		for _, part := range parts {
			if d, ok := data[part].(map[string]interface{}); ok {
				data = d
			} else {
				return nil, ErrSecretNotFound
			}
		}
	} else {
		// 兼容旧逻辑：自动检测 data 字段
		if d, ok := result["data"].(map[string]interface{}); ok {
			data = d
		}
	}

	// 提取密钥信息，使用字段映射
	secret := &model.Secret{
		AuthType: p.authType,
		Username: "root",
	}

	// 提取 username
	usernamePath := p.config.FieldMapping.Username
	if usernamePath == "" {
		usernamePath = "username"
	}
	if v := p.extractField(data, usernamePath); v != "" {
		secret.Username = v
	}

	// 根据 auth_type 提取对应字段
	switch p.authType {
	case "ssh_key":
		keyPath := p.config.FieldMapping.PrivateKey
		if keyPath == "" {
			keyPath = "private_key"
		}
		secret.PrivateKey = p.extractField(data, keyPath)
	case "password":
		pwdPath := p.config.FieldMapping.Password
		if pwdPath == "" {
			pwdPath = "password"
		}
		secret.Password = p.extractField(data, pwdPath)
	}

	// 检查是否获取到有效凭据
	if secret.PrivateKey == "" && secret.Password == "" {
		return nil, ErrSecretNotFound
	}

	return secret, nil
}

// extractField 从 data 中提取字段（支持点分隔路径）
func (p *WebhookProvider) extractField(data map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			if v, ok := current[part].(string); ok {
				return v
			}
			return ""
		}
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return ""
		}
	}
	return ""
}

// TestConnection 测试连接
func (p *WebhookProvider) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", p.config.URL, nil)
	if err != nil {
		return err
	}

	p.applyAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接 Webhook 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("Webhook 服务错误: HTTP %d", resp.StatusCode)
	}

	return nil
}

// Name 获取提供者名称
func (p *WebhookProvider) Name() string {
	return p.name
}
