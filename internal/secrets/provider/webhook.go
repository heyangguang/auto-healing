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
	req, err := p.newSecretRequest(ctx, query)
	if err != nil {
		return nil, err
	}
	resp, err := p.executeSecretRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := p.decodeSecretResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	return buildMappedSecret(p.authType, p.config.FieldMapping, func(path string) string {
		return extractStringPath(data, path)
	})
}

func (p *WebhookProvider) newSecretRequest(ctx context.Context, query model.SecretQuery) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, p.config.Method, p.secretURL(query), nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	p.applyAuth(req)
	return req, nil
}

func (p *WebhookProvider) secretURL(query model.SecretQuery) string {
	url := strings.TrimSuffix(p.config.URL, "/")
	switch p.config.QueryKey {
	case "ip":
		return url + "/" + query.IPAddress
	case "hostname":
		return url + "/" + query.Hostname
	default:
		return url
	}
}

func (p *WebhookProvider) executeSecretRequest(req *http.Request) (*http.Response, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Webhook 请求失败: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrSecretNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Webhook 返回错误: HTTP %d, %s", resp.StatusCode, string(body))
	}
	return resp, nil
}

func (p *WebhookProvider) decodeSecretResponse(body io.Reader) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 Webhook 响应失败: %w", err)
	}
	if p.config.ResponseDataPath == "" {
		if data, ok := result["data"].(map[string]interface{}); ok {
			return data, nil
		}
		return result, nil
	}
	return resolveWebhookResponseData(result, p.config.ResponseDataPath)
}

func resolveWebhookResponseData(result map[string]interface{}, path string) (map[string]interface{}, error) {
	current := result
	for _, part := range strings.Split(path, ".") {
		next, ok := current[part].(map[string]interface{})
		if !ok {
			return nil, ErrSecretNotFound
		}
		current = next
	}
	return current, nil
}

// extractField 从 data 中提取字段（支持点分隔路径）
func (p *WebhookProvider) extractField(data map[string]interface{}, path string) string {
	return extractStringPath(data, path)
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
