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
	if err := validateSecretAuthType(source.AuthType); err != nil {
		return nil, err
	}

	var config model.WebhookConfig
	configBytes, _ := json.Marshal(source.Config)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, newConfigError(fmt.Sprintf("解析 Webhook 配置失败: %v", err))
	}

	if config.URL == "" {
		return nil, newConfigError("Webhook URL 不能为空")
	}
	if err := validateProviderURL(config.URL, "Webhook URL"); err != nil {
		return nil, err
	}
	if config.Method == "" {
		return nil, newConfigError("请求方法 (method) 不能为空")
	}
	config.Method = strings.ToUpper(config.Method)
	if config.QueryKey == "" {
		return nil, newConfigError("查询键 (query_key) 不能为空")
	}
	if config.QueryKey != "ip" && config.QueryKey != "hostname" {
		return nil, newConfigError("query_key 只支持 'ip' 或 'hostname'")
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
			return nil, newConfigError("basic 认证需要 username 和 password")
		}
	case "bearer":
		if config.Auth.Token == "" {
			return nil, newConfigError("bearer 认证需要 token")
		}
	case "api_key":
		if config.Auth.HeaderName == "" || config.Auth.APIKey == "" {
			return nil, newConfigError("api_key 认证需要 header_name 和 api_key")
		}
	default:
		return nil, newConfigError(fmt.Sprintf("不支持的 Webhook 认证类型: %s（支持: none, basic, bearer, api_key）", config.Auth.Type))
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
		return nil, newConfigError(fmt.Sprintf("Webhook URL 无效: %v", err))
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
		return nil, newConnectionError("Webhook 请求失败", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrSecretNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, newHTTPStatusError("Webhook", resp.StatusCode)
	}
	return resp, nil
}

func (p *WebhookProvider) decodeSecretResponse(body io.Reader) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, newInvalidResponseError("解析 Webhook 响应失败", err)
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
			return nil, newInvalidResponseError("Webhook 响应缺少配置的 response_data_path", nil)
		}
		current = next
	}
	return current, nil
}

// TestConnection 测试连接
func (p *WebhookProvider) TestConnection(ctx context.Context) error {
	resp, err := p.connectionProbe(ctx, http.MethodHead)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
		resp.Body.Close()
		resp, err = p.connectionProbe(ctx, p.config.Method)
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()
	return evaluateWebhookProbe(resp)
}

func (p *WebhookProvider) connectionProbe(ctx context.Context, method string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, p.config.URL, nil)
	if err != nil {
		return nil, newConfigError(fmt.Sprintf("Webhook URL 无效: %v", err))
	}
	p.applyAuth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, newConnectionError("连接 Webhook 失败", err)
	}
	return resp, nil
}

func evaluateWebhookProbe(resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return newAuthError(fmt.Sprintf("Webhook 认证失败 (HTTP %d)", resp.StatusCode), nil)
	case resp.StatusCode >= 500:
		return newRequestError(fmt.Sprintf("Webhook 服务错误 (HTTP %d)", resp.StatusCode), nil)
	case resp.StatusCode >= 400:
		return newRequestError(fmt.Sprintf("Webhook 连接测试失败 (HTTP %d)", resp.StatusCode), nil)
	default:
		return nil
	}
}

// Name 获取提供者名称
func (p *WebhookProvider) Name() string {
	return p.name
}
