package httpapi

import (
	"errors"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
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
	if tenantID, ok := repository.TenantIDFromContextOK(c.Request.Context()); ok {
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

func authTenantIDOrError(c *gin.Context) (uuid.UUID, error) {
	tenantID := currentTenantOrNil(c)
	if tenantID == uuid.Nil {
		return uuid.Nil, repository.ErrTenantContextRequired
	}
	if err := ensureAuthTenantAccessible(c, tenantID); err != nil {
		return uuid.Nil, err
	}
	return tenantID, nil
}

func ensureAuthTenantAccessible(c *gin.Context, tenantID uuid.UUID) error {
	tenant, err := repository.NewTenantRepository().GetByID(c.Request.Context(), tenantID)
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
	if err := ensureAuthTenantMembership(c, tenantID); err != nil {
		return err
	}
	return nil
}

func ensureAuthTenantMembership(c *gin.Context, tenantID uuid.UUID) error {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		return errAuthTenantAccess
	}
	tenants, queryErr := repository.NewTenantRepository().GetUserTenants(c.Request.Context(), userID, "")
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
