package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/query"
	platformhttp "github.com/company/auto-healing/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func parsePagination(c *gin.Context, defaultPageSize int) (int, int) {
	return platformhttp.ParsePagination(c, defaultPageSize)
}

func parsePositiveIntQuery(c *gin.Context, key string, defaultValue, maxValue int) int {
	return platformhttp.ParsePositiveIntQuery(c, key, defaultValue, maxValue)
}

func GetStringFilter(c *gin.Context, field string) query.StringFilter {
	return platformhttp.GetStringFilter(c, field)
}

func respondInternalError(c *gin.Context, sub, publicMsg string, err error) {
	platformhttp.RespondInternalError(c, sub, publicMsg, err)
}

func requireTenantID(c *gin.Context, sub string) (uuid.UUID, bool) {
	return platformhttp.RequireTenantID(c, sub)
}

func FormatValidationError(err error) string {
	return platformhttp.FormatValidationError(err)
}

func ToBusinessError(err error) string {
	return platformhttp.ToBusinessError(err)
}
