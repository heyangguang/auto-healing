package httpapi

import (
	"fmt"

	"github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *UserHandler) resolveCreateUserRoleIDs(c *gin.Context, roleID *uuid.UUID, roleIDs []uuid.UUID) ([]uuid.UUID, error) {
	if roleID != nil && len(roleIDs) > 0 {
		return nil, fmt.Errorf("role_id 和 role_ids 不能同时传递")
	}
	if len(roleIDs) > 0 {
		return h.validatePlatformRoleIDs(c, roleIDs)
	}
	defaultRoleID, err := h.platformRoleID(c, roleID)
	if err != nil {
		return nil, err
	}
	return []uuid.UUID{defaultRoleID}, nil
}

func (h *UserHandler) validatePlatformRoleIDs(c *gin.Context, roleIDs []uuid.UUID) ([]uuid.UUID, error) {
	uniqueRoleIDs := dedupeRoleIDs(roleIDs)
	validated := make([]uuid.UUID, 0, len(uniqueRoleIDs))
	for _, candidate := range uniqueRoleIDs {
		role, err := h.validatePlatformRole(c, &candidate)
		if err != nil {
			return nil, err
		}
		validated = append(validated, role.ID)
	}
	return validated, nil
}

func (h *UserHandler) platformRoleID(c *gin.Context, roleID *uuid.UUID) (uuid.UUID, error) {
	if roleID != nil {
		role, err := h.validatePlatformRole(c, roleID)
		if err != nil {
			return uuid.Nil, err
		}
		return role.ID, nil
	}

	platformAdminRole, err := h.roleRepo.GetByName(c.Request.Context(), "platform_admin")
	if err != nil {
		return uuid.Nil, errDefaultPlatformRole
	}
	return platformAdminRole.ID, nil
}

func (h *UserHandler) validatePlatformRole(c *gin.Context, roleID *uuid.UUID) (*model.Role, error) {
	if roleID == nil {
		return nil, nil
	}
	role, err := h.roleRepo.GetByID(c.Request.Context(), *roleID)
	if err != nil {
		return nil, fmt.Errorf("指定的角色不存在")
	}
	if role.Scope != "platform" {
		return nil, fmt.Errorf("只能分配平台级别角色")
	}
	return role, nil
}

func (h *UserHandler) validatePlatformAdminMutation(c *gin.Context, userID uuid.UUID, status string, targetRole *model.Role) error {
	lastAdmin, err := h.isLastPlatformAdmin(c, userID)
	if err != nil {
		return fmt.Errorf("%w: %v", errPlatformAdminStateCheck, err)
	}
	if status != "" && status != "active" && lastAdmin {
		return fmt.Errorf("系统中必须保留至少一个可用的平台管理员，无法禁用")
	}
	if targetRole != nil && targetRole.Name != "platform_admin" && lastAdmin {
		return fmt.Errorf("系统中必须保留至少一个平台管理员，无法变更角色")
	}
	return nil
}

func applyPlatformUserUpdate(user *model.User, req *UpdateUserRequest) {
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Status != "" {
		user.Status = req.Status
	}
}

func (h *UserHandler) respondCreatedUser(c *gin.Context, userID uuid.UUID, fallback *model.User) {
	userWithRoles, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		respondInternalError(c, "USER", "重新加载用户失败", err)
		return
	}
	response.Created(c, chooseUserResponse(userWithRoles, fallback))
}

func (h *UserHandler) respondUpdatedUser(c *gin.Context, userID uuid.UUID, fallback *model.User) {
	userWithRoles, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		respondInternalError(c, "USER", "重新加载用户失败", err)
		return
	}
	response.Success(c, chooseUserResponse(userWithRoles, fallback))
}

func chooseUserResponse(reloaded, fallback *model.User) *model.User {
	if reloaded != nil {
		return reloaded
	}
	return fallback
}

func (h *UserHandler) validateAssignedPlatformRoles(c *gin.Context, userID uuid.UUID, roleIDs []uuid.UUID) ([]uuid.UUID, error) {
	validatedRoleIDs, err := h.validatePlatformRoleIDs(c, roleIDs)
	if err != nil {
		return nil, err
	}
	lastAdmin, err := h.isLastPlatformAdmin(c, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errPlatformAdminStateCheck, err)
	}
	if !lastAdmin {
		return validatedRoleIDs, nil
	}
	for _, roleID := range validatedRoleIDs {
		if role, err := h.roleRepo.GetByID(c.Request.Context(), roleID); err == nil && role.Name == "platform_admin" {
			return validatedRoleIDs, nil
		}
	}
	return nil, fmt.Errorf("系统中必须保留至少一个平台管理员，无法移除 platform_admin 角色")
}

func (h *UserHandler) isProtectedPlatformAdmin(c *gin.Context, userID uuid.UUID) (bool, error) {
	platformAdmins, _, err := h.userRepo.List(c.Request.Context(), &accessrepo.UserListParams{
		Page:         1,
		PageSize:     2,
		PlatformOnly: true,
	})
	if err != nil {
		return false, err
	}
	if len(platformAdmins) > 1 {
		return false, nil
	}
	for _, user := range platformAdmins {
		if user.ID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (h *UserHandler) isLastPlatformAdmin(c *gin.Context, userID uuid.UUID) (bool, error) {
	platformAdmins, _, err := h.userRepo.List(c.Request.Context(), &accessrepo.UserListParams{
		Page:         1,
		PageSize:     2,
		PlatformOnly: true,
		Status:       "active",
	})
	if err != nil {
		return false, err
	}
	if len(platformAdmins) > 1 {
		return false, nil
	}
	return len(platformAdmins) == 1 && platformAdmins[0].ID == userID, nil
}

func dedupeRoleIDs(roleIDs []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(roleIDs))
	result := make([]uuid.UUID, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		result = append(result, roleID)
	}
	return result
}
