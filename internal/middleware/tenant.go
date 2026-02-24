package middleware

import (
	"net/http"
	"strings"

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

		if isPlatformAdmin {
			// ===== 平台管理员 =====
			// 必须通过 Impersonation 才能访问租户级 API
			if IsImpersonating(c) {
				// Impersonation 模式：使用申请单指定的租户 ID
				impTenantIDStr, _ := c.Get(ImpersonationTenantIDKey)
				if impTenantIDStr == nil || impTenantIDStr.(string) == "" {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"code":    40300,
						"message": "Impersonation 会话缺少租户信息",
					})
					return
				}
				parsed, err := uuid.Parse(impTenantIDStr.(string))
				if err != nil {
					c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
						"code":    40000,
						"message": "Impersonation 租户 ID 格式无效",
					})
					return
				}
				tenantID = parsed
			} else if tenantIDStr == "" {
				// 平台管理员未指定租户 + 未 Impersonation
				// 只允许访问不需要租户上下文的路由，其余一律拒绝
				path := c.Request.URL.Path
				if platformAdminAllowedWithoutTenant(path) {
					c.Next()
					return
				}
				// 租户级路由 — 必须通过 Impersonation 才能访问
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":    40300,
					"message": "此接口为租户级资源，平台管理员需通过临时提权（Impersonation）后才能访问",
				})
				return
			} else {
				// 平台管理员指定了租户但未 Impersonation → 拒绝
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":    40300,
					"message": "平台管理员需通过 Impersonation 审批后才能访问租户数据",
				})
				return
			}
		} else {
			// ===== 普通用户 =====
			if tenantIDStr == "" {
				// 未指定租户 → 使用默认租户
				if defaultTenantID == "" {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"code":    40300,
						"message": "用户未分配任何租户，请联系管理员",
					})
					return
				}
				tenantID = uuid.MustParse(defaultTenantID)
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
				if !contains(tenantIDs, tenantIDStr) {
					// JWT 缓存未命中 → 回退到数据库实时查询
					// 场景：管理员将用户添加到新租户后，用户的旧 JWT 不包含该租户
					userIDStr := GetUserID(c)
					uid, parseErr := uuid.Parse(userIDStr)
					if parseErr != nil {
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
							"code":    40300,
							"message": "无权访问该租户",
						})
						return
					}
					tenantRepo := repository.NewTenantRepository()
					dbTenants, dbErr := tenantRepo.GetUserTenants(c.Request.Context(), uid, "")
					if dbErr != nil || !containsTenantByID(dbTenants, tenantIDStr) {
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
							"code":    40300,
							"message": "无权访问该租户",
						})
						return
					}
					// 数据库确认有权限 → 放行，并通知前端刷新 Token 以更新缓存
					c.Header("X-Refresh-Token", "true")
				}
				tenantID = parsed
			}
		}

		// 🔒 验证租户是否处于 active 状态（禁用的租户不允许访问）
		tenantRepo := repository.NewTenantRepository()
		tenant, tenantErr := tenantRepo.GetByID(c.Request.Context(), tenantID)
		if tenantErr != nil || tenant == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    40300,
				"message": "租户不存在",
			})
			return
		}
		if tenant.Status != model.TenantStatusActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    40301,
				"message": "该租户已被禁用，请联系平台管理员",
			})
			return
		}

		c.Set(TenantIDKey, tenantID.String())

		// 将 tenantID 注入到 Go context 中，供 Repository 层 TenantDB() 使用
		ctx := repository.WithTenantID(c.Request.Context(), tenantID)
		c.Request = c.Request.WithContext(ctx)

		// 🔄 实时从数据库加载该租户下的权限和角色（覆盖 JWT 缓存）
		// 这样管理员修改角色权限后，用户刷新页面即可生效，无需重新登录
		// Impersonation 模式跳过：由 ImpersonationMiddleware 独立控制权限
		if !isPlatformAdmin && !IsImpersonating(c) {
			userIDStr := GetUserID(c)
			if uid, parseErr := uuid.Parse(userIDStr); parseErr == nil {
				permRepo := repository.NewPermissionRepository()
				if dbPerms, permErr := permRepo.GetTenantPermissionCodes(c.Request.Context(), uid, tenantID); permErr == nil {
					c.Set(PermissionsKey, dbPerms)
				}
			}
		}

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

// containsTenantByID 检查租户列表中是否包含指定 ID 的租户
func containsTenantByID(tenants []model.Tenant, targetID string) bool {
	for _, t := range tenants {
		if t.ID.String() == targetID {
			return true
		}
	}
	return false
}

// platformAdminAllowedWithoutTenant 判断平台管理员在未提权时是否可以访问该路由
// 白名单：只有不需要租户上下文的通用接口才放行
func platformAdminAllowedWithoutTenant(path string) bool {
	allowedPrefixes := []string{
		"/api/v1/auth/",        // 认证相关（me, profile, logout）
		"/api/v1/user/",        // 用户偏好、收藏、最近访问、租户列表
		"/api/v1/platform/",    // 平台管理接口
		"/api/v1/dictionaries", // 字典查询（全局共享）
		"/api/v1/search",       // 全局搜索
		"/api/v1/workbench/",   // 工作台概览
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
