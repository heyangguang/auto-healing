package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/middleware"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	respPkg "github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ownedScopeTestContext struct {
	userID          string
	defaultTenantID string
	permissions     []string
}

func newOwnedScopeTestRouter(ctx ownedScopeTestContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if ctx.userID != "" {
			c.Set(middleware.UserIDKey, ctx.userID)
		}
		if ctx.defaultTenantID != "" {
			c.Set(middleware.DefaultTenantIDKey, ctx.defaultTenantID)
			c.Set(middleware.TenantIDKey, ctx.defaultTenantID)
		}
		if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
			c.Set(middleware.TenantIDKey, tenantID)
		}
		if ctx.permissions != nil {
			c.Set(middleware.PermissionsKey, ctx.permissions)
		}
		if tenantID, exists := c.Get(middleware.TenantIDKey); exists {
			tenantUUID, err := uuid.Parse(tenantID.(string))
			if err != nil {
				panic(err)
			}
			c.Request = c.Request.WithContext(accessrepo.WithTenantID(c.Request.Context(), tenantUUID))
		}
		c.Next()
	})
	return router
}

func decodeOwnedScopeResponse(t *testing.T, recorder *httptest.ResponseRecorder) respPkg.Response {
	t.Helper()
	var resp respPkg.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return resp
}
