package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/query"
	platformhttp "github.com/company/auto-healing/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SearchableField = platformhttp.SearchableField

type FilterOption = platformhttp.FilterOption

type resourceErrorMode = platformhttp.ResourceErrorMode

const (
	resourceErrorModeInternal   = platformhttp.ResourceErrorModeInternal
	resourceErrorModeBadRequest = platformhttp.ResourceErrorModeBadRequest
)

func GetStringFilter(c *gin.Context, field string) query.StringFilter {
	return platformhttp.GetStringFilter(c, field)
}

func BuildSchemaScopes(c *gin.Context, schema []SearchableField, excludeKeys ...string) []func(*gorm.DB) *gorm.DB {
	return platformhttp.BuildSchemaScopes(c, schema, excludeKeys...)
}

func parsePagination(c *gin.Context, defaultPageSize int) (int, int) {
	return platformhttp.ParsePagination(c, defaultPageSize)
}

func parsePositiveIntQuery(c *gin.Context, key string, defaultValue, maxValue int) int {
	return platformhttp.ParsePositiveIntQuery(c, key, defaultValue, maxValue)
}

func getQueryInt(c *gin.Context, key string, defaultValue int) int {
	return platformhttp.ParseIntQuery(c, key, defaultValue)
}

func respondInternalError(c *gin.Context, sub, publicMsg string, err error) {
	platformhttp.RespondInternalError(c, sub, publicMsg, err)
}

func respondResourceError(c *gin.Context, sub, publicMsg, notFoundMsg string, notFoundErr error, mode resourceErrorMode, err error) {
	platformhttp.RespondResourceError(c, sub, publicMsg, notFoundMsg, notFoundErr, mode, err)
}

func ToBusinessError(err error) string {
	return platformhttp.ToBusinessError(err)
}
