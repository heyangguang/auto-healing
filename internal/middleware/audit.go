package middleware

import (
	"github.com/gin-gonic/gin"
)

// AuditMiddleware 审计中间件 — 自动记录写操作的审计日志
func AuditMiddleware() gin.HandlerFunc {
	return AuditMiddlewareWithDeps(NewRuntimeDeps())
}

func AuditMiddlewareWithDeps(deps RuntimeDeps) gin.HandlerFunc {
	deps = deps.withDefaults()
	return func(c *gin.Context) {
		state, ok := prepareAuditRequest(c, deps.DB)
		if !ok {
			c.Next()
			return
		}
		c.Writer = state.bodyWriter
		c.Next()
		writeAuditLogs(state, captureAuditActor(c, state.bodyWriter), deps.AuditRepo, deps.PlatformAuditRepo, deps.DB)
	}
}
