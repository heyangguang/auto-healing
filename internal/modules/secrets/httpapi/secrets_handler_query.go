package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// QuerySecret 查询密钥
func (h *SecretsHandler) QuerySecret(c *gin.Context) {
	if !middleware.HasPermission(middleware.GetPermissions(c), "secrets:query") {
		middleware.AbortPermissionDenied(c, "secrets:query", "all")
		return
	}

	var query model.SecretQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	secret, err := h.svc.QuerySecret(c.Request.Context(), query)
	if err != nil {
		writeSecretQueryError(c, err)
		return
	}
	response.Success(c, secret)
}

// TestQuery 测试能否从密钥源获取指定主机的凭据（支持单选/多选）
func (h *SecretsHandler) TestQuery(c *gin.Context) {
	id, ok := parseSecretsSourceID(c)
	if !ok {
		return
	}

	var req TestQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	hosts, ok := resolveSecretsTestHosts(c, req)
	if !ok {
		return
	}

	results := make([]TestQueryResult, 0, len(hosts))
	successCount := 0
	for _, host := range hosts {
		result := h.testQueryHost(c, id, host)
		if result.Success {
			successCount++
		}
		results = append(results, result)
	}
	response.Success(c, TestQueryResponse{Results: results, SuccessCount: successCount, FailCount: len(results) - successCount})
}

func resolveSecretsTestHosts(c *gin.Context, req TestQueryRequest) ([]TestQueryHost, bool) {
	switch {
	case len(req.Hosts) > 0:
		return req.Hosts, true
	case req.Hostname != "" || req.IPAddress != "":
		return []TestQueryHost{{Hostname: req.Hostname, IPAddress: req.IPAddress}}, true
	default:
		response.BadRequest(c, "请提供 hostname/ip_address 或 hosts 数组")
		return nil, false
	}
}

func (h *SecretsHandler) testQueryHost(c *gin.Context, id uuid.UUID, host TestQueryHost) TestQueryResult {
	result := TestQueryResult{Hostname: host.Hostname, IPAddress: host.IPAddress}
	secret, err := h.svc.TestQuery(c.Request.Context(), id, host.Hostname, host.IPAddress)
	if err != nil {
		result.Message = publicSecretQueryErrorMessage(err)
		return result
	}

	result.Success = true
	result.AuthType = secret.AuthType
	result.Username = secret.Username
	result.HasCredential = true
	result.Message = "成功获取凭据"
	return result
}
