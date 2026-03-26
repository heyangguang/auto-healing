package repository

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListWithFilter 带过滤条件获取角色列表
func (r *RoleRepository) ListWithFilter(ctx context.Context, f RoleFilter) ([]model.Role, error) {
	queryBuilder := r.db.WithContext(ctx).Preload("Permissions", rolePermissionPreloadScope(f.Scope))
	if f.Name != "" {
		like := "%" + f.Name + "%"
		queryBuilder = queryBuilder.Where("name ILIKE ? OR display_name ILIKE ? OR description ILIKE ?", like, like, like)
	}
	if f.Scope != "" {
		queryBuilder = queryBuilder.Where("scope = ?", f.Scope)
	}
	if f.Scope == "tenant" && f.TenantID != uuid.Nil {
		queryBuilder = queryBuilder.Where("(is_system = true AND tenant_id IS NULL) OR tenant_id = ?", f.TenantID)
	}

	var roles []model.Role
	err := queryBuilder.Order("is_system DESC, created_at ASC").Find(&roles).Error
	return roles, err
}

func rolePermissionPreloadScope(scope string, args ...interface{}) func(*gorm.DB) *gorm.DB {
	if scope != "tenant" {
		return func(db *gorm.DB) *gorm.DB {
			return db
		}
	}
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("code NOT LIKE ?", platformPermissionCodePrefix+"%")
	}
}

// GetRoleUsers 获取角色下的关联用户（分页 + 搜索）
func (r *RoleRepository) GetRoleUsers(ctx context.Context, roleID uuid.UUID, page, pageSize int, name string) ([]RoleUserInfo, int64, error) {
	return r.listRoleUsers(ctx, "user_platform_roles", "user_platform_roles.role_id = ?", []any{roleID}, page, pageSize, name)
}

// GetTenantRoleUsers 获取租户下角色关联的用户（分页 + 搜索）
func (r *RoleRepository) GetTenantRoleUsers(ctx context.Context, roleID, tenantID uuid.UUID, page, pageSize int, name string) ([]RoleUserInfo, int64, error) {
	return r.listRoleUsers(ctx, "user_tenant_roles", "user_tenant_roles.role_id = ? AND user_tenant_roles.tenant_id = ?", []any{roleID, tenantID}, page, pageSize, name)
}

func (r *RoleRepository) listRoleUsers(ctx context.Context, joinTable, predicate string, args []any, page, pageSize int, name string) ([]RoleUserInfo, int64, error) {
	queryBuilder := r.db.WithContext(ctx).
		Table("users").
		Joins("INNER JOIN "+joinTable+" ON "+joinTable+".user_id = users.id").
		Where(predicate, args...)
	if name != "" {
		like := "%" + name + "%"
		queryBuilder = queryBuilder.Where("users.username ILIKE ? OR users.display_name ILIKE ?", like, like)
	}

	total, err := countWithSession(queryBuilder)
	if err != nil {
		return nil, 0, err
	}
	var users []RoleUserInfo
	err = queryBuilder.Select("users.id, users.username, users.display_name, users.email, users.status").
		Order("users.created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error
	return users, total, err
}
