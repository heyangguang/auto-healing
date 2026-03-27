package httpapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// TestConnectionRequest 测试连接请求
type TestConnectionRequest struct {
	SecretsSourceID string `json:"secrets_source_id" binding:"required"`
}

// ConnectionTestResult 连接测试结果
type ConnectionTestResult struct {
	CMDBID    string `json:"cmdb_id"`
	Host      string `json:"host"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	AuthType  string `json:"auth_type,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

// BatchTestConnectionRequest 批量测试连接请求
type BatchTestConnectionRequest struct {
	CMDBIDs         []string `json:"cmdb_ids" binding:"required,min=1,max=50"`
	SecretsSourceID string   `json:"secrets_source_id" binding:"required"`
}

// TestConnection 测试单个 CMDB 配置项的 SSH 连接
func (h *CMDBHandler) TestConnection(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID格式")
		return
	}

	var req TestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	item, err := h.cmdbSvc.GetCMDBItem(c.Request.Context(), id)
	if err != nil {
		respondCMDBItemError(c, "获取配置项失败", err)
		return
	}
	response.Success(c, h.testSSHConnection(c.Request.Context(), id.String(), item.IPAddress, cmdbConnectionHostname(item), req.SecretsSourceID))
}

// BatchTestConnection 批量测试 CMDB 配置项的 SSH 连接
func (h *CMDBHandler) BatchTestConnection(c *gin.Context) {
	var req BatchTestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	results := make([]ConnectionTestResult, 0, len(req.CMDBIDs))
	for _, idStr := range req.CMDBIDs {
		results = append(results, h.testSingleCMDBConnection(c, idStr, req.SecretsSourceID))
	}

	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}
	response.Success(c, gin.H{"total": len(results), "success": successCount, "failed": len(results) - successCount, "results": results})
}

func (h *CMDBHandler) testSingleCMDBConnection(c *gin.Context, idStr, secretsSourceID string) ConnectionTestResult {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return ConnectionTestResult{CMDBID: idStr, Success: false, Message: "无效的 CMDB ID"}
	}
	item, err := h.cmdbSvc.GetCMDBItem(c.Request.Context(), id)
	if err != nil {
		return ConnectionTestResult{CMDBID: idStr, Success: false, Message: cmdbLookupFailureMessage(err)}
	}
	return h.testSSHConnection(c.Request.Context(), idStr, item.IPAddress, cmdbConnectionHostname(item), secretsSourceID)
}

func cmdbConnectionHostname(item *model.CMDBItem) string {
	if item.Hostname != "" {
		return item.Hostname
	}
	return item.Name
}

func (h *CMDBHandler) testSSHConnection(ctx context.Context, cmdbID, ipAddress, hostname, secretsSourceID string) ConnectionTestResult {
	result := ConnectionTestResult{CMDBID: cmdbID, Host: ipAddress}

	secret, err := secrets.NewService().QuerySecret(ctx, model.SecretQuery{
		SourceID:  secretsSourceID,
		Hostname:  hostname,
		IPAddress: ipAddress,
	})
	if err != nil {
		result.Message = "获取凭据失败: " + err.Error()
		return result
	}

	result.AuthType = secret.AuthType
	startTime := time.Now()
	sshErr := testConnectionByAuthType(ipAddress, secret.Username, secret.Password, secret.PrivateKey, secret.AuthType)
	result.LatencyMs = time.Since(startTime).Milliseconds()
	if sshErr != nil {
		result.Message = "连接失败: " + sshErr.Error()
		return result
	}

	result.Success = true
	result.Message = "连接成功"
	return result
}

func testConnectionByAuthType(ipAddress, username, password, privateKey, authType string) error {
	if authType == "ssh_key" {
		return testSSHWithKey(ipAddress, username, privateKey)
	}
	if authType == "password" {
		return testSSHWithPassword(ipAddress, username, password)
	}
	return fmt.Errorf("不支持的认证方式: %s", authType)
}

func testSSHWithKey(host, username, privateKey string) error {
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return fmt.Errorf("解析私钥失败: %v", err)
	}
	config, err := newSSHClientConfig(username, ssh.PublicKeys(signer))
	if err != nil {
		return err
	}
	return dialSSH(host, config)
}

func testSSHWithPassword(host, username, password string) error {
	config, err := newSSHClientConfig(username, ssh.Password(password))
	if err != nil {
		return err
	}
	return dialSSH(host, config)
}

func newSSHClientConfig(username string, authMethod ssh.AuthMethod) (*ssh.ClientConfig, error) {
	callback, err := loadSSHHostKeyCallback()
	if err != nil {
		return nil, err
	}
	return &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: callback,
		Timeout:         5 * time.Second,
	}, nil
}

func dialSSH(host string, config *ssh.ClientConfig) error {
	addr := host
	if !strings.Contains(host, ":") {
		addr = host + ":22"
	}
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	return client.Close()
}

func loadSSHHostKeyCallback() (ssh.HostKeyCallback, error) {
	path, err := resolveKnownHostsPath()
	if err != nil {
		return nil, err
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("加载 known_hosts 失败: %w", err)
	}
	return callback, nil
}

func resolveKnownHostsPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("AUTO_HEALING_KNOWN_HOSTS")); path != "" {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法定位 known_hosts: %w", err)
	}
	path := filepath.Join(homeDir, ".ssh", "known_hosts")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("缺少 known_hosts，请设置 AUTO_HEALING_KNOWN_HOSTS 或在 %s 提供 known_hosts", path)
	}
	return path, nil
}
