package handler

import (
	"strconv"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
)

// SearchHandler 全局搜索处理器
type SearchHandler struct {
	repo *repository.SearchRepository
}

// NewSearchHandler 创建全局搜索处理器
func NewSearchHandler() *SearchHandler {
	return &SearchHandler{
		repo: repository.NewSearchRepository(),
	}
}

// GlobalSearch 全局搜索
// GET /api/v1/search?q={keyword}&limit={limit}
func (h *SearchHandler) GlobalSearch(c *gin.Context) {
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

	results, totalCount, err := h.repo.GlobalSearch(c.Request.Context(), keyword, limit)
	if err != nil {
		response.InternalError(c, "搜索失败: "+err.Error())
		return
	}

	response.Success(c, map[string]interface{}{
		"results":     results,
		"total_count": totalCount,
	})
}
