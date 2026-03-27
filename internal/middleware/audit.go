package middleware

import "github.com/gin-gonic/gin"

func AuditMiddlewareWithDeps(deps RuntimeDeps) gin.HandlerFunc {
	db := deps.requireDB()
	auditRepo := deps.requireAuditRepo()
	platformAuditRepo := deps.requirePlatformAuditRepo()
	return func(c *gin.Context) {
		state, ok := prepareAuditRequest(c, db)
		if !ok {
			c.Next()
			return
		}
		c.Writer = state.bodyWriter
		c.Next()
		writeAuditLogs(state, captureAuditActor(c, state.bodyWriter), auditRepo, platformAuditRepo, db)
	}
}
