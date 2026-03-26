package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/model"
)

// VaultProvider HashiCorp Vault 密钥提供者
type VaultProvider struct {
	config   model.VaultConfig
	authType string // 从 SecretsSource.AuthType 获取
	name     string
	client   *http.Client
	token    string // 运行时获取的 token（AppRole 方式）
}

// NewVaultProvider 创建 Vault 密钥提供者
func NewVaultProvider(source *model.SecretsSource) (*VaultProvider, error) {
	var config model.VaultConfig
	configBytes, _ := json.Marshal(source.Config)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("解析 Vault 配置失败: %w", err)
	}

	if config.Address == "" {
		return nil, fmt.Errorf("Vault 地址不能为空")
	}
	if config.SecretPath == "" {
		return nil, fmt.Errorf("secret_path 不能为空")
	}

	// 验证认证配置
	switch config.Auth.Type {
	case "token":
		if config.Auth.Token == "" {
			return nil, fmt.Errorf("Token 不能为空")
		}
	case "approle":
		if config.Auth.RoleID == "" || config.Auth.SecretID == "" {
			return nil, fmt.Errorf("AppRole 需要 role_id 和 secret_id")
		}
	default:
		return nil, fmt.Errorf("不支持的认证类型: %s（支持: token, approle）", config.Auth.Type)
	}

	return &VaultProvider{
		config:   config,
		authType: source.AuthType,
		name:     source.Name,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// getToken 获取 Vault Token
func (p *VaultProvider) getToken(ctx context.Context) (string, error) {
	switch p.config.Auth.Type {
	case "token":
		return p.config.Auth.Token, nil
	case "approle":
		return p.loginWithAppRole(ctx)
	default:
		return "", fmt.Errorf("不支持的认证类型: %s", p.config.Auth.Type)
	}
}

// loginWithAppRole 使用 AppRole 登录获取 Token
func (p *VaultProvider) loginWithAppRole(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/v1/auth/approle/login", p.config.Address)

	body := map[string]string{
		"role_id":   p.config.Auth.RoleID,
		"secret_id": p.config.Auth.SecretID,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建 AppRole 登录请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AppRole 登录请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AppRole 登录失败: HTTP %d, %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 AppRole 响应失败: %w", err)
	}

	if result.Auth.ClientToken == "" {
		return "", fmt.Errorf("AppRole 登录未返回 Token")
	}

	return result.Auth.ClientToken, nil
}

// GetSecret 获取密钥
func (p *VaultProvider) GetSecret(ctx context.Context, query model.SecretQuery) (*model.Secret, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := p.newSecretRequest(ctx, query, token)
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

func (p *VaultProvider) newSecretRequest(ctx context.Context, query model.SecretQuery, token string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.secretURL(query), nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)
	if p.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", p.config.Namespace)
	}
	return req, nil
}

func (p *VaultProvider) secretURL(query model.SecretQuery) string {
	path := p.config.SecretPath
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	switch p.config.QueryKey {
	case "ip":
		path += "/" + query.IPAddress
	case "hostname":
		path += "/" + query.Hostname
	}
	return fmt.Sprintf("%s/v1/%s", p.config.Address, path)
}

func (p *VaultProvider) executeSecretRequest(req *http.Request) (*http.Response, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Vault 失败: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrSecretNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Vault 返回错误: HTTP %d, %s", resp.StatusCode, string(body))
	}
	return resp, nil
}

func (p *VaultProvider) decodeSecretResponse(body io.Reader) (map[string]interface{}, error) {
	var result struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 Vault 响应失败: %w", err)
	}
	return result.Data.Data, nil
}

// TestConnection 测试连接
func (p *VaultProvider) TestConnection(ctx context.Context) error {
	// 先尝试获取 Token（验证认证配置）
	token, err := p.getToken(ctx)
	if err != nil {
		return fmt.Errorf("认证失败: %w", err)
	}

	url := fmt.Sprintf("%s/v1/sys/health", p.config.Address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Vault-Token", token)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接 Vault 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusTooManyRequests {
		return fmt.Errorf("Vault 健康检查失败: HTTP %d", resp.StatusCode)
	}

	return nil
}

// Name 获取提供者名称
func (p *VaultProvider) Name() string {
	return p.name
}

// extractField 从 data 中提取字段（支持点分隔路径）
func (p *VaultProvider) extractField(data map[string]interface{}, path string) string {
	return extractStringPath(data, path)
}
