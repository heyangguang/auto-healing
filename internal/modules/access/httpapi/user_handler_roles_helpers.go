package httpapi

import (
	"strings"

	"github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *RoleHandler) buildRoleStats(c *gin.Context, roles []model.Role) ([]RoleWithStats, error) {
	stats, err := h.roleRepo.GetRoleStats(c.Request.Context())
	if err != nil {
		return nil, err
	}
	result := make([]RoleWithStats, len(roles))
	for i, role := range roles {
		result[i] = buildRoleWithStats(role, stats[role.ID.String()].UserCount)
	}
	return result, nil
}

func (h *RoleHandler) buildTenantRoleStats(c *gin.Context, roles []model.Role, tenantID uuid.UUID) ([]RoleWithStats, error) {
	stats, err := h.roleRepo.GetTenantRoleStats(c.Request.Context(), tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]RoleWithStats, len(roles))
	for i, role := range roles {
		result[i] = buildRoleWithStats(role, stats[role.ID.String()].UserCount)
	}
	return result, nil
}

func buildRoleWithStats(role model.Role, userCount int64) RoleWithStats {
	return RoleWithStats{
		Role:            role,
		UserCount:       userCount,
		PermissionCount: int64(len(role.Permissions)),
	}
}

func isTenantRoleRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.FullPath(), "/api/v1/tenant/roles")
}

func (h *RoleHandler) getScopedRole(c *gin.Context, id uuid.UUID) (*model.Role, error) {
	if isTenantRoleRequest(c) {
		tenantID, ok := requireTenantID(c, "ROLE")
		if !ok {
			return nil, platformrepo.ErrTenantContextRequired
		}
		return h.roleRepo.GetTenantRoleByID(c.Request.Context(), tenantID, id)
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		return nil, err
	}
	if role.Scope != "platform" {
		return nil, accessrepo.ErrRoleNotFound
	}
	return role, nil
}

func normalizeRoleUsersPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
