package middleware

import (
	"net/http"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	TenantIDKey        = "tenant_id"
	IsPlatformAdminKey = "is_platform_admin"
)

// TenantMiddleware 租户上下文中间件
// 从 X-Tenant-ID 请求头中读取租户 ID，并验证用户是否有权访问该租户。
// 如果未提供，则使用用户的默认租户（第一个租户）。
// 平台管理员可以访问任意租户。
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户的租户列表（来自 JWT Claims）
		var tenantIDs []string
		if rawTenantIDs, exists := c.Get(TenantIDsKey); exists && rawTenantIDs != nil {
			tenantIDs, _ = rawTenantIDs.([]string)
		}

		// 获取用户的默认租户
		defaultTenantID := ""
		if raw, exists := c.Get(DefaultTenantIDKey); exists && raw != nil {
			defaultTenantID, _ = raw.(string)
		}

		// 检查是否为平台管理员
		isPlatformAdmin := IsPlatformAdmin(c)

		// 从请求头获取目标租户 ID
		tenantIDStr := c.GetHeader("X-Tenant-ID")

		var tenantID uuid.UUID

		if tenantIDStr == "" {
			// 未指定租户 → 使用用户的默认租户
			if defaultTenantID == "" {
				if isPlatformAdmin {
					// 平台管理员没有默认租户时，取系统中第一个租户
					tenantRepo := repository.NewTenantRepository()
					tenants, _, err := tenantRepo.List(c.Request.Context(), "", 1, 1)
					if err == nil && len(tenants) > 0 {
						tenantID = tenants[0].ID
					} else {
						// 没有任何租户，跳过租户校验
						c.Next()
						return
					}
				} else {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"code":    40300,
						"message": "用户未分配任何租户，请联系管理员",
					})
					return
				}
			} else {
				tenantID = uuid.MustParse(defaultTenantID)
			}
		} else {
			// 指定了租户 → 验证格式
			parsed, err := uuid.Parse(tenantIDStr)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"code":    40000,
					"message": "无效的 X-Tenant-ID 格式",
				})
				return
			}

			// 验证用户是否有权访问该租户
			if !isPlatformAdmin && !contains(tenantIDs, tenantIDStr) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":    40300,
					"message": "无权访问该租户",
				})
				return
			}

			tenantID = parsed
		}

		c.Set(TenantIDKey, tenantID.String())

		// 将 tenantID 注入到 Go context 中，供 Repository 层 TenantDB() 使用
		ctx := repository.WithTenantID(c.Request.Context(), tenantID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// RequirePlatformAdmin 要求平台管理员权限
// 用于保护平台级管理操作（如租户 CRUD、平台设置等）
func RequirePlatformAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsPlatformAdmin(c) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":    40300,
			"message": "此操作需要平台管理员权限",
		})
	}
}

// GetTenantID 从上下文获取当前租户 ID
func GetTenantID(c *gin.Context) string {
	if id, exists := c.Get(TenantIDKey); exists {
		return id.(string)
	}
	return model.DefaultTenantID.String()
}

// GetTenantUUID 从上下文获取当前租户 UUID
func GetTenantUUID(c *gin.Context) uuid.UUID {
	idStr := GetTenantID(c)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return model.DefaultTenantID
	}
	return id
}

// IsPlatformAdmin 检查当前用户是否为平台管理员
func IsPlatformAdmin(c *gin.Context) bool {
	if v, exists := c.Get(IsPlatformAdminKey); exists {
		return v.(bool)
	}
	return false
}

// contains 检查字符串切片是否包含指定值
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
