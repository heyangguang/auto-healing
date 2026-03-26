package handler

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func detachedTimeoutContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), timeout)
}

func optionalAuthTenantContext() gin.HandlerFunc {
	return authTenantContext(false)
}

func requiredAuthTenantContext() gin.HandlerFunc {
	return authTenantContext(true)
}

func authTenantContext(required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, inject, ok := resolveAuthTenantContext(c, required)
		if !ok {
			return
		}
		if inject {
			c.Set(middleware.TenantIDKey, tenantID.String())
			c.Request = c.Request.WithContext(repository.WithTenantID(c.Request.Context(), tenantID))
		}
		c.Next()
	}
}

func resolveAuthTenantContext(c *gin.Context, required bool) (uuid.UUID, bool, bool) {
	if tenantID, ok := repository.TenantIDFromContextOK(c.Request.Context()); ok {
		return tenantID, true, true
	}
	if middleware.IsImpersonating(c) {
		return resolveOptionalImpersonationTenant(c)
	}
	if middleware.IsPlatformAdmin(c) {
		return uuid.Nil, false, true
	}
	return resolveRegularAuthTenant(c, required)
}

func resolveOptionalImpersonationTenant(c *gin.Context) (uuid.UUID, bool, bool) {
	raw, exists := c.Get(middleware.ImpersonationTenantIDKey)
	tenantID, err := uuid.Parse(stringContextValue(raw, exists))
	if err != nil {
		response.InternalError(c, "Impersonation 租户上下文缺失")
		c.Abort()
		return uuid.Nil, false, false
	}
	return tenantID, true, true
}

func resolveRegularAuthTenant(c *gin.Context, required bool) (uuid.UUID, bool, bool) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr != "" {
		return resolveRequestedTenant(c, tenantIDStr)
	}
	return resolveDefaultTenant(c, required)
}

func resolveDefaultTenant(c *gin.Context, required bool) (uuid.UUID, bool, bool) {
	defaultTenantID := stringContextValue(c.Get(middleware.DefaultTenantIDKey))
	if defaultTenantID == "" {
		if required {
			response.Forbidden(c, "用户未分配任何租户，请联系管理员")
			c.Abort()
			return uuid.Nil, false, false
		}
		return uuid.Nil, false, true
	}
	tenantID, err := uuid.Parse(defaultTenantID)
	if err != nil {
		if required {
			response.Forbidden(c, "默认租户无效，请重新登录")
			c.Abort()
			return uuid.Nil, false, false
		}
		return uuid.Nil, false, true
	}
	ok, checkOK := currentUserHasTenant(c, tenantID.String(), required)
	if !checkOK {
		return uuid.Nil, false, false
	}
	if !ok {
		if required {
			response.Forbidden(c, "无权访问该租户")
			c.Abort()
			return uuid.Nil, false, false
		}
		return uuid.Nil, false, true
	}
	return tenantID, true, true
}

func resolveRequestedTenant(c *gin.Context, tenantIDStr string) (uuid.UUID, bool, bool) {
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		response.BadRequest(c, "无效的 X-Tenant-ID 格式")
		c.Abort()
		return uuid.Nil, false, false
	}
	ok, checkOK := currentUserHasTenant(c, tenantIDStr, true)
	if !checkOK {
		return uuid.Nil, false, false
	}
	if !ok {
		response.Forbidden(c, "无权访问该租户")
		c.Abort()
		return uuid.Nil, false, false
	}
	return tenantID, true, true
}

func currentUserHasTenant(c *gin.Context, tenantID string, failOnError bool) (bool, bool) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		if failOnError {
			response.InternalError(c, "解析当前用户失败")
			c.Abort()
			return false, false
		}
		return false, true
	}
	tenants, err := repository.NewTenantRepository().GetUserTenants(c.Request.Context(), userID, "")
	if err != nil {
		if failOnError {
			response.InternalError(c, "加载租户上下文失败")
			c.Abort()
			return false, false
		}
		return false, true
	}
	for _, tenant := range tenants {
		if tenant.ID.String() == tenantID {
			return true, true
		}
	}
	return false, true
}

func stringContextValue(value interface{}, exists bool) string {
	if !exists || value == nil {
		return ""
	}
	str, _ := value.(string)
	return str
}
