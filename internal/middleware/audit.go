package middleware

import (
	"github.com/company/auto-healing/internal/database"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/gin-gonic/gin"
)

// AuditMiddleware 审计中间件 — 自动记录写操作的审计日志
func AuditMiddleware() gin.HandlerFunc {
	repo := auditrepo.NewAuditLogRepository(database.DB)
	platformRepo := auditrepo.NewPlatformAuditLogRepository()
	db := database.DB

	return func(c *gin.Context) {
		state, ok := prepareAuditRequest(c, db)
		if !ok {
			c.Next()
			return
		}
		c.Writer = state.bodyWriter
		c.Next()
		writeAuditLogs(state, captureAuditActor(c, state.bodyWriter), repo, platformRepo, db)
	}
}
