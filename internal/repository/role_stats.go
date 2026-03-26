package repository

import (
	"context"

	"github.com/google/uuid"
)

// GetRoleStats 获取所有角色的统计信息（用户数 + 权限数）
func (r *RoleRepository) GetRoleStats(ctx context.Context) (map[string]RoleStats, error) {
	stats := make(map[string]RoleStats)
	if err := r.loadPlatformRoleUserCounts(ctx, stats); err != nil {
		return nil, err
	}
	if err := r.loadRolePermissionCounts(ctx, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// GetTenantRoleStats 获取角色统计信息（按租户隔离的用户数 + 权限数）
func (r *RoleRepository) GetTenantRoleStats(ctx context.Context, tenantID uuid.UUID) (map[string]RoleStats, error) {
	stats := make(map[string]RoleStats)
	if err := r.loadTenantRoleUserCounts(ctx, tenantID, stats); err != nil {
		return nil, err
	}
	if err := r.loadRolePermissionCounts(ctx, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *RoleRepository) loadPlatformRoleUserCounts(ctx context.Context, stats map[string]RoleStats) error {
	type userCountResult struct {
		RoleID    string `gorm:"column:role_id"`
		UserCount int64  `gorm:"column:user_count"`
	}
	var userCounts []userCountResult
	if err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.id as role_id, COUNT(DISTINCT user_platform_roles.user_id) as user_count").
		Joins("LEFT JOIN user_platform_roles ON user_platform_roles.role_id = roles.id").
		Group("roles.id").
		Find(&userCounts).Error; err != nil {
		return err
	}

	for _, item := range userCounts {
		stats[item.RoleID] = RoleStats{RoleID: item.RoleID, UserCount: item.UserCount}
	}
	return nil
}

func (r *RoleRepository) loadTenantRoleUserCounts(ctx context.Context, tenantID uuid.UUID, stats map[string]RoleStats) error {
	type userCountResult struct {
		RoleID    string `gorm:"column:role_id"`
		UserCount int64  `gorm:"column:user_count"`
	}
	var userCounts []userCountResult
	if err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.id as role_id, COUNT(DISTINCT user_tenant_roles.user_id) as user_count").
		Joins("LEFT JOIN user_tenant_roles ON user_tenant_roles.role_id = roles.id AND user_tenant_roles.tenant_id = ?", tenantID).
		Group("roles.id").
		Find(&userCounts).Error; err != nil {
		return err
	}

	for _, item := range userCounts {
		stats[item.RoleID] = RoleStats{RoleID: item.RoleID, UserCount: item.UserCount}
	}
	return nil
}

func (r *RoleRepository) loadRolePermissionCounts(ctx context.Context, stats map[string]RoleStats) error {
	type permCountResult struct {
		RoleID    string `gorm:"column:role_id"`
		PermCount int64  `gorm:"column:perm_count"`
	}
	var permCounts []permCountResult
	if err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.id as role_id, COUNT(DISTINCT role_permissions.permission_id) as perm_count").
		Joins("LEFT JOIN role_permissions ON role_permissions.role_id = roles.id").
		Group("roles.id").
		Find(&permCounts).Error; err != nil {
		return err
	}

	for _, item := range permCounts {
		stat := stats[item.RoleID]
		stat.RoleID = item.RoleID
		stat.PermissionCount = item.PermCount
		stats[item.RoleID] = stat
	}
	return nil
}
