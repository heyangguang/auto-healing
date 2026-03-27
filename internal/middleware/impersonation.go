package middleware

import (
	"context"
	"errors"
	"sync"
	"time"

	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// ImpersonationKey gin.Context 中标记当前请求是否为 Impersonation
	ImpersonationKey          = "is_impersonating"
	ImpersonationRequestIDKey = "impersonation_request_id"
	ImpersonationTenantIDKey  = "impersonation_tenant_id"
)

const (
	ErrorCodeImpersonationPlatformOnly        = "IMPERSONATION_PLATFORM_ONLY"
	ErrorCodeImpersonationRequestMissing      = "IMPERSONATION_REQUEST_ID_MISSING"
	ErrorCodeImpersonationRequestInvalid      = "IMPERSONATION_REQUEST_ID_INVALID"
	ErrorCodeImpersonationRequestNotFound     = "IMPERSONATION_REQUEST_NOT_FOUND"
	ErrorCodeImpersonationRequestLookupFailed = "IMPERSONATION_REQUEST_LOOKUP_FAILED"
	ErrorCodeImpersonationUserMismatch        = "IMPERSONATION_USER_MISMATCH"
	ErrorCodeImpersonationSessionInvalid      = "IMPERSONATION_SESSION_INVALID"
	ErrorCodeImpersonationTenantMismatch      = "IMPERSONATION_TENANT_MISMATCH"
	ErrorCodeImpersonationPermsLoadFailed     = "IMPERSONATION_PERMS_LOAD_FAILED"
)

// 缓存 impersonation_accessor 角色的权限列表（进程级缓存）
var (
	impersonationPermsMu       sync.RWMutex
	impersonationPerms         []string
	impersonationPermsLoadedAt time.Time
	impersonationPermsTTL      = 30 * time.Second
	impersonationPermsLoader   = loadImpersonationPermissionsFromDB
)

// loadImpersonationPermissions 从数据库加载 impersonation_accessor 角色的权限
func loadImpersonationPermissions(ctx context.Context) ([]string, error) {
	return loadImpersonationPermissionsWithLoader(ctx, impersonationPermsLoader)
}

func loadImpersonationPermissionsWithLoader(ctx context.Context, loader func(context.Context) ([]string, error)) ([]string, error) {
	now := time.Now()

	impersonationPermsMu.RLock()
	if len(impersonationPerms) > 0 && now.Sub(impersonationPermsLoadedAt) < impersonationPermsTTL {
		cached := append([]string(nil), impersonationPerms...)
		impersonationPermsMu.RUnlock()
		return cached, nil
	}
	impersonationPermsMu.RUnlock()

	perms, err := loader(ctx)
	if err != nil {
		return nil, err
	}

	impersonationPermsMu.Lock()
	impersonationPerms = append([]string(nil), perms...)
	impersonationPermsLoadedAt = now
	impersonationPermsMu.Unlock()

	return append([]string(nil), perms...), nil
}

func loadImpersonationPermissionsFromDB(ctx context.Context) ([]string, error) {
	roleRepo := accessrepo.NewRoleRepository()
	return loadImpersonationPermissionsFromRoleRepo(ctx, roleRepo)
}

func loadImpersonationPermissionsFromRoleRepo(ctx context.Context, roleRepo *accessrepo.RoleRepository) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	role, err := roleRepo.GetByName(ctx, "impersonation_accessor")
	if err != nil {
		return nil, err
	}
	codes := make([]string, len(role.Permissions))
	for i, p := range role.Permissions {
		codes[i] = p.Code
	}
	return codes, nil
}

// ImpersonationMiddleware 验证 Impersonation 会话
// 当检测到请求携带 X-Impersonation=true 时：
// 1. 从 X-Impersonation-Request-ID 获取申请单 ID
// 2. 验证申请单 status=active 且未过期
// 3. 验证 requester_id 与当前用户匹配
// 4. 在 gin.Context 中设置 impersonation 标记
// 5. 用 impersonation_accessor 角色权限覆盖 JWT 中的 * 通配符
func ImpersonationMiddleware() gin.HandlerFunc {
	return ImpersonationMiddlewareWithDeps(NewRuntimeDeps())
}

func ImpersonationMiddlewareWithDeps(deps RuntimeDeps) gin.HandlerFunc {
	deps = deps.withDefaults()
	return func(c *gin.Context) {
		c.Set(ImpersonationKey, false)
		if !requestIsImpersonating(c) {
			c.Next()
			return
		}
		if !IsPlatformAdmin(c) {
			abortForbidden(c, "只有平台管理员才能使用 Impersonation", ErrorCodeImpersonationPlatformOnly)
			return
		}
		requestID, ok := parseImpersonationRequestID(c)
		if !ok {
			return
		}
		req, ok := loadActiveImpersonationRequest(c, deps.ImpersonationRepo, requestID)
		if !ok {
			return
		}
			if !applyImpersonationContextWithLoader(c, requestID, req.TenantID, func(innerCtx context.Context) ([]string, error) {
				return loadImpersonationPermissionsFromRoleRepo(innerCtx, deps.RoleRepo)
			}) {
				return
			}
			c.Next()
	}
}

func requestIsImpersonating(c *gin.Context) bool {
	return c.GetHeader("X-Impersonation") == "true"
}

func parseImpersonationRequestID(c *gin.Context) (uuid.UUID, bool) {
	requestIDStr := c.GetHeader("X-Impersonation-Request-ID")
	if requestIDStr == "" {
		abortBadRequest(c, "Impersonation 缺少 X-Impersonation-Request-ID", ErrorCodeImpersonationRequestMissing)
		return uuid.Nil, false
	}
	requestID, err := uuid.Parse(requestIDStr)
	if err != nil {
		abortBadRequest(c, "无效的 X-Impersonation-Request-ID", ErrorCodeImpersonationRequestInvalid)
		return uuid.Nil, false
	}
	return requestID, true
}

func loadActiveImpersonationRequest(c *gin.Context, repo *accessrepo.ImpersonationRepository, requestID uuid.UUID) (*accessmodel.ImpersonationRequest, bool) {
	req, err := repo.GetByID(c.Request.Context(), requestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			abortForbidden(c, "Impersonation 申请不存在", ErrorCodeImpersonationRequestNotFound)
			return nil, false
		}
		abortInternalError(c, "加载 Impersonation 申请失败", ErrorCodeImpersonationRequestLookupFailed)
		return nil, false
	}
	if req == nil {
		abortForbidden(c, "Impersonation 申请不存在", ErrorCodeImpersonationRequestNotFound)
		return nil, false
	}
	if !validateImpersonationRequest(c, req) {
		return nil, false
	}
	return req, true
}

func validateImpersonationRequest(c *gin.Context, req *accessmodel.ImpersonationRequest) bool {
	userID, _ := uuid.Parse(GetUserID(c))
	if req.RequesterID != userID {
		abortForbidden(c, "Impersonation 申请与当前用户不匹配", ErrorCodeImpersonationUserMismatch)
		return false
	}
	if !req.IsSessionValid() {
		abortForbidden(c, "Impersonation 会话已过期或未激活", ErrorCodeImpersonationSessionInvalid)
		return false
	}
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr != "" && tenantIDStr != req.TenantID.String() {
		abortForbidden(c, "Impersonation 请求的租户与申请不匹配", ErrorCodeImpersonationTenantMismatch)
		return false
	}
	return true
}

func applyImpersonationContext(c *gin.Context, requestID, tenantID uuid.UUID) bool {
	return applyImpersonationContextWithLoader(c, requestID, tenantID, loadImpersonationPermissions)
}

func applyImpersonationContextWithLoader(c *gin.Context, requestID, tenantID uuid.UUID, loader func(context.Context) ([]string, error)) bool {
	perms, err := loadImpersonationPermissionsWithLoader(c.Request.Context(), loader)
	if err != nil {
		logger.Auth("IMPERSONATION").Error("加载 impersonation 权限失败: request=%s err=%v", requestID, err)
		abortInternalError(c, "加载 Impersonation 权限失败", ErrorCodeImpersonationPermsLoadFailed)
		return false
	}
	c.Set(ImpersonationKey, true)
	c.Set(ImpersonationRequestIDKey, requestID.String())
	c.Set(ImpersonationTenantIDKey, tenantID.String())
	c.Set(PermissionsKey, perms)
	return true
}

// IsImpersonating 检查当前请求是否为 Impersonation
func IsImpersonating(c *gin.Context) bool {
	if v, exists := c.Get(ImpersonationKey); exists {
		return v.(bool)
	}
	return false
}
