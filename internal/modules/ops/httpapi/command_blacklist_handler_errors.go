package httpapi

import (
	"errors"
	"strconv"
	"strings"

	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func buildCommandBlacklistListOptions(c *gin.Context, page, pageSize int) (*opsrepo.CommandBlacklistListOptions, error) {
	opts := &opsrepo.CommandBlacklistListOptions{
		Page:         page,
		PageSize:     pageSize,
		Name:         c.Query("name"),
		NameExact:    c.Query("name__exact"),
		Category:     c.Query("category"),
		Severity:     c.Query("severity"),
		Pattern:      c.Query("pattern"),
		PatternExact: c.Query("pattern__exact"),
	}
	active, err := parseOptionalBoolQuery(c.Query("is_active"))
	if err != nil {
		return nil, err
	}
	opts.IsActive = active
	return opts, nil
}

func parseOptionalBoolQuery(raw string) (*bool, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, errors.New("is_active 必须是合法布尔值")
	}
	return &parsed, nil
}

func respondCommandBlacklistError(c *gin.Context, publicMsg string, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.NotFound(c, "规则不存在")
	case errors.Is(err, platformrepo.ErrTenantContextRequired):
		respondInternalError(c, "BLACKLIST", publicMsg, err)
	case isCommandBlacklistBadRequest(err):
		response.BadRequest(c, err.Error())
	default:
		respondInternalError(c, "BLACKLIST", publicMsg, err)
	}
}

func isCommandBlacklistBadRequest(err error) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "系统内置规则不可删除":
		return true
	default:
		return hasCommandBlacklistValidationPrefix(err.Error())
	}
}

func hasCommandBlacklistValidationPrefix(message string) bool {
	for _, prefix := range []string{"无效的匹配类型", "无效的正则表达式", "无效的严重级别"} {
		if strings.HasPrefix(message, prefix) {
			return true
		}
	}
	return false
}
