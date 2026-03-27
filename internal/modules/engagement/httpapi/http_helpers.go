package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/pkg/query"
	platformhttp "github.com/company/auto-healing/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetStringFilter(c *gin.Context, field string) query.StringFilter {
	return platformhttp.GetStringFilter(c, field)
}

func parsePagination(c *gin.Context, defaultPageSize int) (int, int) {
	return platformhttp.ParsePagination(c, defaultPageSize)
}

func respondInternalError(c *gin.Context, sub, publicMsg string, err error) {
	platformhttp.RespondInternalError(c, sub, publicMsg, err)
}

func requireTenantID(c *gin.Context, sub string) (uuid.UUID, bool) {
	return platformhttp.RequireTenantID(c, sub)
}

func requireTenantContext(c *gin.Context, msg string) bool {
	if _, exists := c.Get(middleware.TenantIDKey); exists {
		return true
	}
	response.Forbidden(c, msg)
	return false
}
