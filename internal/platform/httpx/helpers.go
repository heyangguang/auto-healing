package httpx

import (
	"strconv"

	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const MaxPageSize = 100

func ParsePagination(c *gin.Context, defaultPageSize int) (int, int) {
	page := ParsePositiveIntQuery(c, "page", 1, 0)
	pageSize := ParsePositiveIntQuery(c, "page_size", defaultPageSize, MaxPageSize)
	return page, pageSize
}

func ParsePositiveIntQuery(c *gin.Context, key string, defaultValue, maxValue int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	if maxValue > 0 && parsed > maxValue {
		return maxValue
	}
	return parsed
}

func RespondInternalError(c *gin.Context, sub, publicMsg string, err error) {
	if err != nil {
		logger.API(sub).Error("%s: %v", publicMsg, err)
	}
	response.InternalError(c, publicMsg)
}

func RequireTenantID(c *gin.Context, sub string) (uuid.UUID, bool) {
	tenantID, err := platformrepo.RequireTenantID(c.Request.Context())
	if err != nil {
		RespondInternalError(c, sub, "租户上下文缺失", err)
		return uuid.Nil, false
	}
	return tenantID, true
}
