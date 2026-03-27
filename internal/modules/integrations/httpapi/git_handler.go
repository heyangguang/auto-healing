package httpapi

import (
	gitSvc "github.com/company/auto-healing/internal/modules/integrations/service/git"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GitRepoHandler Git 仓库处理器
type GitRepoHandler struct {
	svc *gitSvc.Service
}

type GitRepoHandlerDeps struct {
	Service *gitSvc.Service
}

func NewGitRepoHandlerWithDeps(deps GitRepoHandlerDeps) *GitRepoHandler {
	if deps.Service == nil {
		panic("integrations git handler requires service")
	}
	return &GitRepoHandler{
		svc: deps.Service,
	}
}

func (h *GitRepoHandler) Shutdown() {
	if h == nil || h.svc == nil {
		return
	}
	h.svc.Shutdown()
}

// ==================== Search Schema 声明 ====================

var gitRepoSearchSchema = []SearchableField{
	{Key: "name", Label: "仓库名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "输入仓库名称", Column: "name"},
	{Key: "url", Label: "仓库地址", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "输入仓库 URL", Column: "url"},
	{Key: "status", Label: "仓库状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "就绪", Value: "ready"}, {Label: "待同步", Value: "pending"},
		{Label: "同步中", Value: "syncing"}, {Label: "错误", Value: "error"},
	}},
	{Key: "auth_type", Label: "认证方式", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "公开", Value: "none"}, {Label: "Token", Value: "token"},
		{Label: "密码", Value: "password"}, {Label: "SSH 密钥", Value: "ssh_key"},
	}},
	{Key: "sync_enabled", Label: "定时同步", Type: "boolean", MatchModes: []string{"exact"}, DefaultMode: "exact"},
}

// GetSearchSchema 返回 Git 仓库搜索 schema
func (h *GitRepoHandler) GetSearchSchema(c *gin.Context) {
	response.Success(c, gitRepoSearchSchema)
}
