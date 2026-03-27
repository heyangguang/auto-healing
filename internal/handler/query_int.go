package handler

import (
	platformhttp "github.com/company/auto-healing/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

func getQueryInt(c *gin.Context, key string, defaultValue int) int {
	return platformhttp.ParseIntQuery(c, key, defaultValue)
}
