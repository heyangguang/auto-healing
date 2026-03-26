package handler

import (
	"strconv"

	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxPageSize = 100

func parsePagination(c *gin.Context, defaultPageSize int) (int, int) {
	page := parsePositiveIntQuery(c, "page", 1, 0)
	pageSize := parsePositiveIntQuery(c, "page_size", defaultPageSize, maxPageSize)
	return page, pageSize
}

func parsePositiveIntQuery(c *gin.Context, key string, defaultValue, maxValue int) int {
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

func respondInternalError(c *gin.Context, sub, publicMsg string, err error) {
	if err != nil {
		logger.API(sub).Error("%s: %v", publicMsg, err)
	}
	response.InternalError(c, publicMsg)
}

func requireTenantID(c *gin.Context, sub string) (uuid.UUID, bool) {
	tenantID, err := repository.RequireTenantID(c.Request.Context())
	if err != nil {
		respondInternalError(c, sub, "租户上下文缺失", err)
		return uuid.Nil, false
	}
	return tenantID, true
}
