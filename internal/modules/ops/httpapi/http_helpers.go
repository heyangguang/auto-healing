package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/query"
	platformhttp "github.com/company/auto-healing/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

func GetStringFilter(c *gin.Context, field string) query.StringFilter {
	return platformhttp.GetStringFilter(c, field)
}

func parsePagination(c *gin.Context, defaultPageSize int) (int, int) {
	return platformhttp.ParsePagination(c, defaultPageSize)
}

func parsePositiveIntQuery(c *gin.Context, key string, defaultValue, maxValue int) int {
	return platformhttp.ParsePositiveIntQuery(c, key, defaultValue, maxValue)
}

func respondInternalError(c *gin.Context, sub, publicMsg string, err error) {
	platformhttp.RespondInternalError(c, sub, publicMsg, err)
}

func FormatValidationError(err error) string {
	return platformhttp.FormatValidationError(err)
}
