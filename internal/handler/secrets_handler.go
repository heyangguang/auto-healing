package handler

import (
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	secretsSvc "github.com/company/auto-healing/internal/service/secrets"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SecretsHandler 密钥处理器
type SecretsHandler struct {
	svc *secretsSvc.Service
}

// NewSecretsHandler 创建密钥处理器
func NewSecretsHandler() *SecretsHandler {
	return &SecretsHandler{
		svc: secretsSvc.NewService(),
	}
}

// ListSources 获取密钥源列表
func (h *SecretsHandler) ListSources(c *gin.Context) {
	sourceType := c.Query("type")
	status := c.Query("status")

	// 解析 is_default 参数
	var isDefault *bool
	if isDefaultStr := c.Query("is_default"); isDefaultStr != "" {
		val := isDefaultStr == "true"
		isDefault = &val
	}

	sources, err := h.svc.ListSources(c.Request.Context(), sourceType, status, isDefault)
	if err != nil {
		response.InternalError(c, "获取密钥源列表失败")
		return
	}

	// 隐藏敏感配置
	for i := range sources {
		sources[i].Config = maskConfig(sources[i].Config)
	}

	response.Success(c, sources)
}

// CreateSource 创建密钥源
func (h *SecretsHandler) CreateSource(c *gin.Context) {
	var req CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	source, err := h.svc.CreateSource(c.Request.Context(), req.ToModel())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, source)
}

// GetSource 获取密钥源详情
func (h *SecretsHandler) GetSource(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	source, err := h.svc.GetSource(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "密钥源不存在")
		return
	}

	source.Config = maskConfig(source.Config)
	response.Success(c, source)
}

// UpdateSource 更新密钥源
func (h *SecretsHandler) UpdateSource(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	var req UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	source, err := h.svc.UpdateSource(c.Request.Context(), id, req.Config, req.IsDefault, req.Priority, req.Status)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, source)
}

// DeleteSource 删除密钥源
func (h *SecretsHandler) DeleteSource(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.DeleteSource(c.Request.Context(), id); err != nil {
		response.InternalError(c, "删除失败")
		return
	}

	response.Message(c, "删除成功")
}

// TestConnection 测试连接
func (h *SecretsHandler) TestConnection(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.TestConnection(c.Request.Context(), id); err != nil {
		response.BadRequest(c, "连接测试失败: "+err.Error())
		return
	}

	response.Message(c, "连接测试成功")
}

// QuerySecret 查询密钥
func (h *SecretsHandler) QuerySecret(c *gin.Context) {
	var query model.SecretQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	secret, err := h.svc.QuerySecret(c.Request.Context(), query)
	if err != nil {
		response.NotFound(c, "密钥未找到: "+err.Error())
		return
	}

	response.Success(c, secret)
}

// TestQueryHost 单个主机
type TestQueryHost struct {
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ip_address"`
}

// TestQueryRequest 测试凭据查询请求（支持单选和多选）
type TestQueryRequest struct {
	// 单选模式
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ip_address"`
	// 多选模式
	Hosts []TestQueryHost `json:"hosts"`
}

// TestQueryResult 单个主机测试结果
type TestQueryResult struct {
	Hostname      string `json:"hostname"`
	IPAddress     string `json:"ip_address"`
	Success       bool   `json:"success"`
	AuthType      string `json:"auth_type,omitempty"`
	Username      string `json:"username,omitempty"`
	HasCredential bool   `json:"has_credential"`
	Message       string `json:"message,omitempty"`
}

// TestQueryResponse 测试凭据查询响应
type TestQueryResponse struct {
	Results      []TestQueryResult `json:"results"`
	SuccessCount int               `json:"success_count"`
	FailCount    int               `json:"fail_count"`
}

// TestQuery 测试能否从密钥源获取指定主机的凭据（支持单选/多选）
func (h *SecretsHandler) TestQuery(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	var req TestQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 收集所有主机（兼容单选和多选）
	var hosts []TestQueryHost
	if len(req.Hosts) > 0 {
		hosts = req.Hosts
	} else if req.Hostname != "" || req.IPAddress != "" {
		hosts = []TestQueryHost{{Hostname: req.Hostname, IPAddress: req.IPAddress}}
	} else {
		response.BadRequest(c, "请提供 hostname/ip_address 或 hosts 数组")
		return
	}

	// 批量测试
	var results []TestQueryResult
	successCount := 0

	for _, host := range hosts {
		result := TestQueryResult{
			Hostname:  host.Hostname,
			IPAddress: host.IPAddress,
		}

		secret, err := h.svc.TestQuery(c.Request.Context(), id, host.Hostname, host.IPAddress)
		if err != nil {
			result.Success = false
			result.HasCredential = false
			result.Message = err.Error()
		} else {
			result.Success = true
			result.AuthType = secret.AuthType
			result.Username = secret.Username
			result.HasCredential = true
			result.Message = "成功获取凭据"
			successCount++
		}
		results = append(results, result)
	}

	response.Success(c, TestQueryResponse{
		Results:      results,
		SuccessCount: successCount,
		FailCount:    len(results) - successCount,
	})
}

// Enable 启用密钥源
func (h *SecretsHandler) Enable(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.Enable(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Message(c, "密钥源已启用")
}

// Disable 禁用密钥源
func (h *SecretsHandler) Disable(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.Disable(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Message(c, "密钥源已禁用")
}

// maskConfig 隐藏敏感配置
func maskConfig(config model.JSON) model.JSON {
	masked := make(model.JSON)
	for k, v := range config {
		if k == "token" || k == "password" || k == "secret" || k == "api_key" {
			masked[k] = "***"
		} else {
			masked[k] = v
		}
	}
	return masked
}
