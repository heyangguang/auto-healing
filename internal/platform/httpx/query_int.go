package httpx

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func ParseIntQuery(c *gin.Context, key string, defaultValue int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue
	}
	var result int
	if _, err := fmt.Sscanf(raw, "%d", &result); err == nil {
		return result
	}
	return defaultValue
}
