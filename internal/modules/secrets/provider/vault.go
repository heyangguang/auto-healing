package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
)

// VaultProvider HashiCorp Vault 密钥提供者
type VaultProvider struct {
	config   secretsmodel.VaultConfig
	authType string // 从 SecretsSource.AuthType 获取
	name     string
	client   *http.Client
}

// NewVaultProvider 创建 Vault 密钥提供者
func NewVaultProvider(source *secretsmodel.SecretsSource) (*VaultProvider, error) {
	if err := validateSecretAuthType(source.AuthType); err != nil {
		return nil, err
	}

	var config secretsmodel.VaultConfig
	configBytes, _ := json.Marshal(source.Config)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, newConfigError(fmt.Sprintf("解析 Vault 配置失败: %v", err))
	}

	if config.Address == "" {
		return nil, newConfigError("Vault 地址不能为空")
	}
	if err := validateProviderURL(config.Address, "Vault 地址"); err != nil {
		return nil, err
	}
	if config.SecretPath == "" {
		return nil, newConfigError("secret_path 不能为空")
	}
	if config.QueryKey == "" {
		return nil, newConfigError("query_key 不能为空")
	}
	if config.QueryKey != "ip" && config.QueryKey != "hostname" {
		return nil, newConfigError("query_key 只支持 'ip' 或 'hostname'")
	}

	// 验证认证配置
	switch config.Auth.Type {
	case "token":
		if config.Auth.Token == "" {
			return nil, newConfigError("token 认证需要提供 token")
		}
	case "approle":
		if config.Auth.RoleID == "" || config.Auth.SecretID == "" {
			return nil, newConfigError("approle 认证需要提供 role_id 和 secret_id")
		}
	default:
		return nil, newConfigError(fmt.Sprintf("不支持的 Vault 认证类型: %s（支持: token, approle）", config.Auth.Type))
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
		return "", newConfigError(fmt.Sprintf("不支持的 Vault 认证类型: %s", p.config.Auth.Type))
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
		return "", newConfigError(fmt.Sprintf("Vault 地址无效: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	if p.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", p.config.Namespace)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", newConnectionError("Vault AppRole 登录请求失败", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", newHTTPStatusError("Vault AppRole 登录", resp.StatusCode)
	}

	var result struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", newInvalidResponseError("解析 Vault AppRole 响应失败", err)
	}

	if result.Auth.ClientToken == "" {
		return "", newInvalidResponseError("Vault AppRole 响应缺少 client_token", nil)
	}

	return result.Auth.ClientToken, nil
}

// GetSecret 获取密钥
func (p *VaultProvider) GetSecret(ctx context.Context, query secretsmodel.SecretQuery) (*secretsmodel.Secret, error) {
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

func (p *VaultProvider) newSecretRequest(ctx context.Context, query secretsmodel.SecretQuery, token string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.secretURL(query), nil)
	if err != nil {
		return nil, newConfigError(fmt.Sprintf("Vault 地址无效: %v", err))
	}
	req.Header.Set("X-Vault-Token", token)
	if p.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", p.config.Namespace)
	}
	return req, nil
}

func (p *VaultProvider) secretURL(query secretsmodel.SecretQuery) string {
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
		return nil, newConnectionError("请求 Vault 失败", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrSecretNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, newHTTPStatusError("Vault", resp.StatusCode)
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
		return nil, newInvalidResponseError("解析 Vault 响应失败", err)
	}
	if result.Data.Data == nil {
		return nil, newInvalidResponseError("Vault 响应缺少 data.data", nil)
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
		return newConfigError(fmt.Sprintf("Vault 地址无效: %v", err))
	}

	req.Header.Set("X-Vault-Token", token)
	if p.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", p.config.Namespace)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return newConnectionError("连接 Vault 失败", err)
	}
	defer resp.Body.Close()

	if !isHealthyVaultStatus(resp.StatusCode) {
		return newHTTPStatusError("Vault 健康检查", resp.StatusCode)
	}

	return nil
}

// Name 获取提供者名称
func (p *VaultProvider) Name() string {
	return p.name
}

func isHealthyVaultStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusOK, http.StatusTooManyRequests, 472, 473:
		return true
	default:
		return false
	}
}
