package httpapi

import (
	"errors"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/modules/access/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	errAuthTenantNotFound = errors.New("租户不存在")
	errAuthTenantDisabled = errors.New("该租户已被禁用，请联系平台管理员")
	errAuthTenantAccess   = errors.New("无权访问该租户")
)

func currentTenantOrNil(c *gin.Context) uuid.UUID {
	if tenantID, ok := platformrepo.TenantIDFromContextOK(c.Request.Context()); ok {
		return tenantID
	}
	if tenantID, ok := headerTenantOrNil(c); ok {
		return tenantID
	}
	if tenantID, ok := defaultTenantOrNil(c); ok {
		return tenantID
	}
	return uuid.Nil
}

func (h *AuthHandler) authTenantIDOrError(c *gin.Context) (uuid.UUID, error) {
	tenantID := currentTenantOrNil(c)
	if tenantID == uuid.Nil {
		return uuid.Nil, platformrepo.ErrTenantContextRequired
	}
	if err := h.ensureAuthTenantAccessible(c, tenantID); err != nil {
		return uuid.Nil, err
	}
	return tenantID, nil
}

func (h *AuthHandler) ensureAuthTenantAccessible(c *gin.Context, tenantID uuid.UUID) error {
	tenant, err := h.tenantRepo.GetByID(c.Request.Context(), tenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errAuthTenantNotFound
		}
		return err
	}
	if tenant == nil {
		return errAuthTenantNotFound
	}
	if tenant.Status != model.TenantStatusActive {
		return errAuthTenantDisabled
	}
	if err := h.ensureAuthTenantMembership(c, tenantID); err != nil {
		return err
	}
	return nil
}

func (h *AuthHandler) ensureAuthTenantMembership(c *gin.Context, tenantID uuid.UUID) error {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		return errAuthTenantAccess
	}
	tenants, queryErr := h.tenantRepo.GetUserTenants(c.Request.Context(), userID, "")
	if queryErr != nil {
		return queryErr
	}
	for _, tenant := range tenants {
		if tenant.ID == tenantID {
			return nil
		}
	}
	return errAuthTenantAccess
}

func headerTenantOrNil(c *gin.Context) (uuid.UUID, bool) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" || !containsTenantClaim(c, tenantIDStr) {
		return uuid.Nil, false
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return uuid.Nil, false
	}
	return tenantID, true
}

func defaultTenantOrNil(c *gin.Context) (uuid.UUID, bool) {
	defaultTenantID, exists := c.Get(middleware.DefaultTenantIDKey)
	if !exists || defaultTenantID == nil {
		return uuid.Nil, false
	}
	defaultTenantIDStr, ok := defaultTenantID.(string)
	if !ok || defaultTenantIDStr == "" {
		return uuid.Nil, false
	}
	tenantID, err := uuid.Parse(defaultTenantIDStr)
	if err != nil {
		return uuid.Nil, false
	}
	return tenantID, true
}

func containsTenantClaim(c *gin.Context, tenantID string) bool {
	rawTenantIDs, exists := c.Get(middleware.TenantIDsKey)
	if !exists || rawTenantIDs == nil {
		return false
	}
	tenantIDs, ok := rawTenantIDs.([]string)
	if !ok {
		return false
	}
	for _, current := range tenantIDs {
		if current == tenantID {
			return true
		}
	}
	return false
}
