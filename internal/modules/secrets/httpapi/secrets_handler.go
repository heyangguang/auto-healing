package httpapi

import secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"

// SecretsHandler 密钥处理器
type SecretsHandler struct {
	svc *secretsSvc.Service
}

type SecretsHandlerDeps struct {
	Service *secretsSvc.Service
}

// NewSecretsHandler 创建密钥处理器
func NewSecretsHandler() *SecretsHandler {
	return NewSecretsHandlerWithDeps(SecretsHandlerDeps{
		Service: secretsSvc.NewService(),
	})
}

func NewSecretsHandlerWithDeps(deps SecretsHandlerDeps) *SecretsHandler {
	return &SecretsHandler{
		svc: deps.Service,
	}
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
