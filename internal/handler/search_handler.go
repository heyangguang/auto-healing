package handler

import (
	"strconv"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
)

// SearchHandler 全局搜索处理器
type SearchHandler struct {
	repo *repository.SearchRepository
}

type SearchHandlerDeps struct {
	Repo *repository.SearchRepository
}

// NewSearchHandler 创建全局搜索处理器
func NewSearchHandler() *SearchHandler {
	return NewSearchHandlerWithDeps(SearchHandlerDeps{
		Repo: repository.NewSearchRepository(),
	})
}

func NewSearchHandlerWithDeps(deps SearchHandlerDeps) *SearchHandler {
	return &SearchHandler{
		repo: deps.Repo,
	}
}

// GlobalSearch 全局搜索
// GET /api/v1/search?q={keyword}&limit={limit}
func (h *SearchHandler) GlobalSearch(c *gin.Context) {
	if !requireTenantContext(c, "当前搜索需要租户上下文，请先选择租户或通过 Impersonation 进入租户") {
		return
	}

	keyword := c.Query("q")
	if keyword == "" {
		response.BadRequest(c, "搜索关键词不能为空")
		return
	}

	limit := 5
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 20 {
			limit = parsed
		}
	}

	results, totalCount, err := h.repo.GlobalSearch(c.Request.Context(), keyword, limit, buildSearchAllowlist(middleware.GetPermissions(c)))
	if err != nil {
		respondInternalError(c, "SEARCH", "搜索失败", err)
		return
	}

	response.Success(c, map[string]interface{}{
		"results":     results,
		"total_count": totalCount,
	})
}

func requireTenantContext(c *gin.Context, msg string) bool {
	if _, exists := c.Get(middleware.TenantIDKey); exists {
		return true
	}
	response.Forbidden(c, msg)
	return false
}

func buildSearchAllowlist(perms []string) map[string]bool {
	allow := make(map[string]bool)
	if middleware.HasPermission(perms, "plugin:list") {
		for _, key := range []string{"hosts", "incidents", "secrets", "plugins"} {
			allow[key] = true
		}
	}
	if middleware.HasPermission(perms, "repository:list") {
		allow["git_repos"] = true
	}
	if middleware.HasPermission(perms, "playbook:list") {
		allow["playbooks"] = true
	}
	if middleware.HasPermission(perms, "healing:rules:view") {
		allow["rules"] = true
	}
	if middleware.HasPermission(perms, "healing:flows:view") {
		allow["flows"] = true
	}
	if middleware.HasPermission(perms, "healing:instances:view") {
		allow["instances"] = true
	}
	if middleware.HasPermission(perms, "task:list") {
		for _, key := range []string{"templates", "schedules", "execution_runs"} {
			allow[key] = true
		}
	}
	if middleware.HasPermission(perms, "template:list") {
		allow["notification_templates"] = true
	}
	if middleware.HasPermission(perms, "channel:list") {
		allow["notification_channels"] = true
	}
	return allow
}
