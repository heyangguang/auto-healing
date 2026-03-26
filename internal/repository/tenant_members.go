package repository

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListMembers 查询租户成员（带角色和用户信息）
func (r *TenantRepository) ListMembers(ctx context.Context, tenantID uuid.UUID) ([]model.UserTenantRole, error) {
	var members []model.UserTenantRole
	err := r.db.WithContext(ctx).
		Preload("Role").
		Preload("Tenant").
		Where("user_tenant_roles.tenant_id = ?", tenantID).
		Find(&members).Error
	if err != nil || len(members) == 0 {
		return members, err
	}

	userIDs := make([]uuid.UUID, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}
	var users []model.User
	if loadErr := r.db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; loadErr == nil {
		userMap := make(map[uuid.UUID]model.User, len(users))
		for _, user := range users {
			userMap[user.ID] = user
		}
		for i := range members {
			if user, ok := userMap[members[i].UserID]; ok {
				members[i].User = user
			}
		}
	}
	return members, nil
}

// ListSimpleMembers 获取租户下简要用户列表
func (r *TenantRepository) ListSimpleMembers(ctx context.Context, tenantID uuid.UUID, search string, status string) ([]SimpleUser, error) {
	var users []SimpleUser
	query := r.db.WithContext(ctx).
		Table("users").
		Select(`users.id, users.username, users.display_name, users.status`).
		Joins("INNER JOIN user_tenant_roles ON user_tenant_roles.user_id = users.id").
		Where("user_tenant_roles.tenant_id = ?", tenantID)
	if status != "" {
		query = query.Where("users.status = ?", status)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("users.username ILIKE ? OR users.display_name ILIKE ?", like, like)
	}
	return users, query.Order("users.username ASC").Limit(500).Scan(&users).Error
}

// AddMember 添加成员到租户
func (r *TenantRepository) AddMember(ctx context.Context, userID, tenantID, roleID uuid.UUID) error {
	if err := r.ensureTenantAssignableRole(ctx, tenantID, roleID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&model.UserTenantRole{
		UserID:   userID,
		TenantID: tenantID,
		RoleID:   roleID,
	}).Error
}

// RemoveMember 从租户移除成员
func (r *TenantRepository) RemoveMember(ctx context.Context, userID, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND tenant_id = ?", userID, tenantID).Delete(&model.UserTenantRole{}).Error
}

// GetMember 查询用户在租户内的角色记录
func (r *TenantRepository) GetMember(ctx context.Context, userID, tenantID uuid.UUID) (*model.UserTenantRole, error) {
	var member model.UserTenantRole
	err := r.db.WithContext(ctx).Where("user_id = ? AND tenant_id = ?", userID, tenantID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// UpdateMemberRole 更新用户在租户内的角色
func (r *TenantRepository) UpdateMemberRole(ctx context.Context, userID, tenantID, roleID uuid.UUID) error {
	if err := r.ensureTenantAssignableRole(ctx, tenantID, roleID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.UserTenantRole{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Update("role_id", roleID).Error
}

func (r *TenantRepository) UpdateMemberUserAndRole(ctx context.Context, user *model.User, tenantID uuid.UUID, roleID *uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(user).Error; err != nil {
			return err
		}
		if roleID == nil {
			return nil
		}
		if err := r.ensureTenantAssignableRole(ctx, tenantID, *roleID); err != nil {
			return err
		}
		return tx.Model(&model.UserTenantRole{}).
			Where("user_id = ? AND tenant_id = ?", user.ID, tenantID).
			Update("role_id", *roleID).Error
	})
}

func (r *TenantRepository) ensureTenantAssignableRole(ctx context.Context, tenantID, roleID uuid.UUID) error {
	if _, err := NewRoleRepositoryWithDB(r.db).GetTenantRoleByID(ctx, tenantID, roleID); err != nil {
		return err
	}
	return nil
}

// GetUserTenants 获取用户所属的租户列表
func (r *TenantRepository) GetUserTenants(ctx context.Context, userID uuid.UUID, search string) ([]model.Tenant, error) {
	var tenants []model.Tenant
	query := r.db.WithContext(ctx).
		Table("tenants").
		Joins("INNER JOIN user_tenant_roles ON user_tenant_roles.tenant_id = tenants.id").
		Where("user_tenant_roles.user_id = ?", userID).
		Where("tenants.status = ?", model.TenantStatusActive)
	if search != "" {
		query = query.Where("tenants.name ILIKE ? OR tenants.code ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	return tenants, query.Group("tenants.id").Order("tenants.id ASC").Find(&tenants).Error
}

// GetUserAllRoles 获取用户在所有租户中的角色（去重）
func (r *TenantRepository) GetUserAllRoles(ctx context.Context, userID uuid.UUID) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).
		Distinct("roles.*").
		Table("roles").
		Joins("INNER JOIN user_tenant_roles ON user_tenant_roles.role_id = roles.id").
		Where("user_tenant_roles.user_id = ?", userID).
		Find(&roles).Error
	return roles, err
}

// GetUserTenantRoles 获取用户在指定租户中的角色
func (r *TenantRepository) GetUserTenantRoles(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).
		Distinct("roles.*").
		Table("roles").
		Joins("INNER JOIN user_tenant_roles ON user_tenant_roles.role_id = roles.id").
		Where("user_tenant_roles.user_id = ? AND user_tenant_roles.tenant_id = ?", userID, tenantID).
		Find(&roles).Error
	return roles, err
}
