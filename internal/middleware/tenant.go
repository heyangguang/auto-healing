package middleware

import (
	"errors"

	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	TenantIDKey        = "tenant_id"
	IsPlatformAdminKey = "is_platform_admin"
)

const (
	ErrorCodeImpersonationTenantMissing = "IMPERSONATION_TENANT_MISSING"
	ErrorCodeImpersonationTenantInvalid = "IMPERSONATION_TENANT_INVALID"
	ErrorCodeImpersonationRequired      = "IMPERSONATION_REQUIRED"
	ErrorCodeTenantUnassigned           = "TENANT_UNASSIGNED"
	ErrorCodeTenantIDInvalid            = "TENANT_ID_INVALID"
	ErrorCodeTenantAccessDenied         = "TENANT_ACCESS_DENIED"
	ErrorCodeTenantNotFound             = "TENANT_NOT_FOUND"
	ErrorCodeTenantLookupFailed         = "TENANT_LOOKUP_FAILED"
	ErrorCodeTenantDisabled             = "TENANT_DISABLED"
	ErrorCodePlatformAdminRequired      = "PLATFORM_ADMIN_REQUIRED"
	ErrorCodeDefaultTenantInvalid       = "DEFAULT_TENANT_INVALID"
)

func ensureActiveTenantWithRepo(c *gin.Context, tenantRepo *accessrepo.TenantRepository, tenantID uuid.UUID) bool {
	tenant, tenantErr := tenantRepo.GetByID(c.Request.Context(), tenantID)
	if tenantErr != nil {
		if errors.Is(tenantErr, gorm.ErrRecordNotFound) {
			abortForbidden(c, "租户不存在", ErrorCodeTenantNotFound)
			return false
		}
		abortInternalError(c, "租户状态校验失败", ErrorCodeTenantLookupFailed)
		return false
	}
	if tenant == nil {
		abortForbidden(c, "租户不存在", ErrorCodeTenantNotFound)
		return false
	}
	if tenant.Status != accessmodel.TenantStatusActive {
		abortForbidden(c, "该租户已被禁用，请联系平台管理员", ErrorCodeTenantDisabled)
		return false
	}
	return true
}

func injectTenantContext(c *gin.Context, tenantID uuid.UUID) {
	c.Set(TenantIDKey, tenantID.String())
	ctx := platformrepo.WithTenantID(c.Request.Context(), tenantID)
	c.Request = c.Request.WithContext(ctx)
}

// GetTenantID 从上下文获取当前租户 ID
func GetTenantID(c *gin.Context) string {
	if id, exists := c.Get(TenantIDKey); exists {
		return id.(string)
	}
	return ""
}

// GetTenantUUID 从上下文获取当前租户 UUID
func GetTenantUUID(c *gin.Context) uuid.UUID {
	idStr := GetTenantID(c)
	if idStr == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
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
func containsTenantByID(tenants []accessmodel.Tenant, targetID string) bool {
	for _, tenant := range tenants {
		if tenant.ID.String() == targetID {
			return true
		}
	}
	return false
}
