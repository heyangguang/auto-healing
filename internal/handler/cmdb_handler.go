package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/service/plugin"
	"github.com/company/auto-healing/internal/service/secrets"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

// CMDBHandler CMDB 处理器
type CMDBHandler struct {
	cmdbSvc *plugin.CMDBService
}

// NewCMDBHandler 创建 CMDB 处理器
func NewCMDBHandler() *CMDBHandler {
	return &CMDBHandler{
		cmdbSvc: plugin.NewCMDBService(),
	}
}

// ListCMDBItems 获取 CMDB 列表
func (h *CMDBHandler) ListCMDBItems(c *gin.Context) {
	page := 1
	pageSize := 20

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}

	itemType := c.Query("type")
	status := c.Query("status")
	environment := c.Query("environment")
	sourcePluginName := c.Query("source_plugin_name")

	// 新增 plugin_id 筛选
	var pluginID *uuid.UUID
	if pidStr := c.Query("plugin_id"); pidStr != "" {
		pid, err := uuid.Parse(pidStr)
		if err == nil {
			pluginID = &pid
		}
	}

	// 新增 has_plugin 筛选
	var hasPlugin *bool
	if hpStr := c.Query("has_plugin"); hpStr != "" {
		hp := hpStr == "true"
		hasPlugin = &hp
	}

	items, total, err := h.cmdbSvc.ListCMDBItems(c.Request.Context(), page, pageSize, pluginID, itemType, status, environment, sourcePluginName, hasPlugin)
	if err != nil {
		response.InternalError(c, "获取 CMDB 列表失败: "+err.Error())
		return
	}

	response.List(c, items, total, page, pageSize)
}

// GetCMDBItem 获取 CMDB 详情
func (h *CMDBHandler) GetCMDBItem(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID格式")
		return
	}

	item, err := h.cmdbSvc.GetCMDBItem(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "配置项不存在: "+err.Error())
		return
	}

	response.Success(c, item)
}

// GetCMDBStats 获取 CMDB 统计信息
func (h *CMDBHandler) GetCMDBStats(c *gin.Context) {
	stats, err := h.cmdbSvc.GetCMDBStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "获取统计信息失败: "+err.Error())
		return
	}

	response.Success(c, stats)
}

// EnterMaintenanceRequest 进入维护模式请求
type EnterMaintenanceRequest struct {
	Reason string `json:"reason" binding:"required"`
	EndAt  string `json:"end_at"` // RFC3339 格式，可选（空表示无限期维护）
}

// EnterMaintenance 进入维护模式
func (h *CMDBHandler) EnterMaintenance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID格式")
		return
	}

	var req EnterMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: reason 必填")
		return
	}

	var endAt *time.Time
	if req.EndAt != "" {
		t, err := time.Parse(time.RFC3339, req.EndAt)
		if err != nil {
			response.BadRequest(c, "end_at 格式错误，请使用 RFC3339 格式")
			return
		}
		endAt = &t
	}

	operator := "admin" // TODO: 从 JWT 获取
	if err := h.cmdbSvc.EnterMaintenance(c.Request.Context(), id, req.Reason, endAt, operator); err != nil {
		response.InternalError(c, "进入维护模式失败: "+err.Error())
		return
	}

	response.Message(c, "配置项已进入维护模式")
}

// ExitMaintenance 退出维护模式
func (h *CMDBHandler) ExitMaintenance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID格式")
		return
	}

	operator := "admin" // TODO: 从 JWT 获取
	if err := h.cmdbSvc.ExitMaintenance(c.Request.Context(), id, "manual", operator); err != nil {
		response.InternalError(c, "退出维护模式失败: "+err.Error())
		return
	}

	response.Message(c, "配置项已恢复正常")
}

// GetMaintenanceLogs 获取维护日志
func (h *CMDBHandler) GetMaintenanceLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID格式")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	logs, total, err := h.cmdbSvc.GetMaintenanceLogs(c.Request.Context(), id, page, pageSize)
	if err != nil {
		response.InternalError(c, "获取维护日志失败: "+err.Error())
		return
	}

	response.Success(c, map[string]interface{}{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// BatchMaintenanceRequest 批量维护请求
type BatchMaintenanceRequest struct {
	IDs    []string `json:"ids" binding:"required"`
	Reason string   `json:"reason" binding:"required"`
	EndAt  string   `json:"end_at"` // 可选，空表示无限期维护
}

// BatchEnterMaintenance 批量进入维护模式
func (h *CMDBHandler) BatchEnterMaintenance(c *gin.Context) {
	var req BatchMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		response.BadRequest(c, "请选择配置项")
		return
	}
	if len(req.IDs) > 100 {
		response.BadRequest(c, "批量操作最多支持 100 个配置项")
		return
	}

	var endAt *time.Time
	if req.EndAt != "" {
		t, err := time.Parse(time.RFC3339, req.EndAt)
		if err != nil {
			response.BadRequest(c, "end_at 格式错误")
			return
		}
		endAt = &t
	}

	operator := "admin"
	successCount := 0
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		if err := h.cmdbSvc.EnterMaintenance(c.Request.Context(), id, req.Reason, endAt, operator); err == nil {
			successCount++
		}
	}

	response.Success(c, map[string]interface{}{
		"total":   len(req.IDs),
		"success": successCount,
		"failed":  len(req.IDs) - successCount,
	})
}

// BatchExitRequest 批量退出维护请求
type BatchExitRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

// BatchExitMaintenance 批量退出维护模式
func (h *CMDBHandler) BatchExitMaintenance(c *gin.Context) {
	var req BatchExitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		response.BadRequest(c, "请选择配置项")
		return
	}
	if len(req.IDs) > 100 {
		response.BadRequest(c, "批量操作最多支持 100 个配置项")
		return
	}

	operator := "admin"
	successCount := 0
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		if err := h.cmdbSvc.ExitMaintenance(c.Request.Context(), id, "manual", operator); err == nil {
			successCount++
		}
	}

	response.Success(c, map[string]interface{}{
		"total":   len(req.IDs),
		"success": successCount,
		"failed":  len(req.IDs) - successCount,
	})
}

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

	// 获取 CMDB 配置项
	item, err := h.cmdbSvc.GetCMDBItem(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "配置项不存在: "+err.Error())
		return
	}

	// 执行连接测试（传入真实的 IP 和 hostname）
	hostname := item.Hostname
	if hostname == "" {
		hostname = item.Name
	}
	result := h.testSSHConnection(c.Request.Context(), id.String(), item.IPAddress, hostname, req.SecretsSourceID)

	response.Success(c, result)
}

// BatchTestConnectionRequest 批量测试连接请求
type BatchTestConnectionRequest struct {
	CMDBIDs         []string `json:"cmdb_ids" binding:"required,min=1,max=50"`
	SecretsSourceID string   `json:"secrets_source_id" binding:"required"`
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
		id, err := uuid.Parse(idStr)
		if err != nil {
			results = append(results, ConnectionTestResult{
				CMDBID:  idStr,
				Success: false,
				Message: "无效的 CMDB ID",
			})
			continue
		}

		// 获取 CMDB 配置项
		item, err := h.cmdbSvc.GetCMDBItem(c.Request.Context(), id)
		if err != nil {
			results = append(results, ConnectionTestResult{
				CMDBID:  idStr,
				Success: false,
				Message: "配置项不存在",
			})
			continue
		}

		// 执行连接测试（传入真实的 IP 和 hostname）
		hostname := item.Hostname
		if hostname == "" {
			hostname = item.Name
		}
		result := h.testSSHConnection(c.Request.Context(), idStr, item.IPAddress, hostname, req.SecretsSourceID)
		results = append(results, result)
	}

	// 统计
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	response.Success(c, gin.H{
		"total":   len(results),
		"success": successCount,
		"failed":  len(results) - successCount,
		"results": results,
	})
}

// testSSHConnection 执行 SSH 连接测试
func (h *CMDBHandler) testSSHConnection(ctx context.Context, cmdbID, ipAddress, hostname, secretsSourceID string) ConnectionTestResult {
	result := ConnectionTestResult{
		CMDBID: cmdbID,
		Host:   ipAddress,
	}

	// 1. 通过 Secrets Source 获取凭据
	secretsSvc := secrets.NewService()
	query := model.SecretQuery{
		SourceID:  secretsSourceID,
		Hostname:  hostname,
		IPAddress: ipAddress,
	}

	secret, err := secretsSvc.QuerySecret(ctx, query)
	if err != nil {
		result.Success = false
		result.Message = "获取凭据失败: " + err.Error()
		return result
	}

	result.AuthType = secret.AuthType

	// 2. 尝试 SSH 连接
	startTime := time.Now()
	var sshErr error

	if secret.AuthType == "ssh_key" {
		sshErr = testSSHWithKey(ipAddress, secret.Username, secret.PrivateKey)
	} else {
		sshErr = testSSHWithPassword(ipAddress, secret.Username, secret.Password)
	}

	result.LatencyMs = time.Since(startTime).Milliseconds()

	if sshErr != nil {
		result.Success = false
		result.Message = "连接失败: " + sshErr.Error()
	} else {
		result.Success = true
		result.Message = "连接成功"
	}

	return result
}

// testSSHWithKey 使用 SSH Key 测试连接
func testSSHWithKey(host, username, privateKey string) error {
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return fmt.Errorf("解析私钥失败: %v", err)
	}

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	addr := host
	if !strings.Contains(host, ":") {
		addr = host + ":22"
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	client.Close()
	return nil
}

// testSSHWithPassword 使用密码测试连接
func testSSHWithPassword(host, username, password string) error {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	addr := host
	if !strings.Contains(host, ":") {
		addr = host + ":22"
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	client.Close()
	return nil
}
