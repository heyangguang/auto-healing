package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RoleRepository 角色数据仓库
type RoleRepository struct {
	db *gorm.DB
}

// NewRoleRepository 创建角色仓库
func NewRoleRepository() *RoleRepository {
	return &RoleRepository{db: database.DB}
}

// Create 创建角色
func (r *RoleRepository) Create(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

// GetByID 根据 ID 获取角色
func (r *RoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).Preload("Permissions").First(&role, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}
	return &role, err
}

// GetByName 根据名称获取角色
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).Preload("Permissions").First(&role, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}
	return &role, err
}

// Update 更新角色
func (r *RoleRepository) Update(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

// Delete 删除角色
func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// 不允许删除系统角色
	var role model.Role
	if err := r.db.WithContext(ctx).First(&role, "id = ?", id).Error; err != nil {
		return err
	}
	if role.IsSystem {
		return errors.New("不能删除系统内置角色")
	}
	return r.db.WithContext(ctx).Delete(&model.Role{}, "id = ?", id).Error
}

// RoleFilter 角色过滤参数
type RoleFilter struct {
	Search string // 模糊搜索 (name/display_name/description)
}

// List 获取角色列表
func (r *RoleRepository) List(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).Preload("Permissions").Order("is_system DESC, created_at ASC").Find(&roles).Error
	return roles, err
}

// ListWithFilter 带过滤条件获取角色列表
func (r *RoleRepository) ListWithFilter(ctx context.Context, f RoleFilter) ([]model.Role, error) {
	query := r.db.WithContext(ctx).Preload("Permissions")
	if f.Search != "" {
		like := "%" + f.Search + "%"
		query = query.Where("name ILIKE ? OR display_name ILIKE ? OR description ILIKE ?", like, like, like)
	}
	var roles []model.Role
	err := query.Order("is_system DESC, created_at ASC").Find(&roles).Error
	return roles, err
}

// RoleStats 角色统计信息
type RoleStats struct {
	RoleID          string `json:"role_id"`
	UserCount       int64  `json:"user_count"`
	PermissionCount int64  `json:"permission_count"`
}

// GetRoleStats 获取所有角色的统计信息（用户数 + 权限数）
func (r *RoleRepository) GetRoleStats(ctx context.Context) (map[string]RoleStats, error) {
	stats := make(map[string]RoleStats)

	// 查询每个角色的用户数
	type UserCountResult struct {
		RoleID    string `gorm:"column:role_id"`
		UserCount int64  `gorm:"column:user_count"`
	}
	var userCounts []UserCountResult
	r.db.WithContext(ctx).
		Table("roles").
		Select("roles.id as role_id, COUNT(DISTINCT user_roles.user_id) as user_count").
		Joins("LEFT JOIN user_roles ON user_roles.role_id = roles.id").
		Group("roles.id").
		Find(&userCounts)

	for _, uc := range userCounts {
		stats[uc.RoleID] = RoleStats{
			RoleID:    uc.RoleID,
			UserCount: uc.UserCount,
		}
	}

	// 查询每个角色的权限数
	type PermCountResult struct {
		RoleID    string `gorm:"column:role_id"`
		PermCount int64  `gorm:"column:perm_count"`
	}
	var permCounts []PermCountResult
	r.db.WithContext(ctx).
		Table("roles").
		Select("roles.id as role_id, COUNT(DISTINCT role_permissions.permission_id) as perm_count").
		Joins("LEFT JOIN role_permissions ON role_permissions.role_id = roles.id").
		Group("roles.id").
		Find(&permCounts)

	for _, pc := range permCounts {
		s := stats[pc.RoleID]
		s.RoleID = pc.RoleID
		s.PermissionCount = pc.PermCount
		stats[pc.RoleID] = s
	}

	return stats, nil
}

// AssignPermissions 为角色分配权限
func (r *RoleRepository) AssignPermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除现有权限关联
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}

		// 添加新权限关联
		for _, permID := range permissionIDs {
			rp := model.RolePermission{
				RoleID:       roleID,
				PermissionID: permID,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// PermissionRepository 权限数据仓库
type PermissionRepository struct {
	db *gorm.DB
}

// NewPermissionRepository 创建权限仓库
func NewPermissionRepository() *PermissionRepository {
	return &PermissionRepository{db: database.DB}
}

// GetByID 根据 ID 获取权限
func (r *PermissionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Permission, error) {
	var perm model.Permission
	err := r.db.WithContext(ctx).First(&perm, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPermissionNotFound
	}
	return &perm, err
}

// GetByCode 根据权限码获取权限
func (r *PermissionRepository) GetByCode(ctx context.Context, code string) (*model.Permission, error) {
	var perm model.Permission
	err := r.db.WithContext(ctx).First(&perm, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPermissionNotFound
	}
	return &perm, err
}

// PermissionFilter 权限过滤参数
type PermissionFilter struct {
	Search string // 全文模糊搜索 (name/code/module)
	Module string // 精确按模块过滤
	Name   string // 模糊搜索 name
	Code   string // 模糊搜索 code
}

// List 获取所有权限
func (r *PermissionRepository) List(ctx context.Context) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.WithContext(ctx).Order("module, resource, action").Find(&perms).Error
	return perms, err
}

// ListWithFilter 带过滤条件获取权限
func (r *PermissionRepository) ListWithFilter(ctx context.Context, f PermissionFilter) ([]model.Permission, error) {
	query := r.db.WithContext(ctx)

	if f.Search != "" {
		like := "%" + f.Search + "%"
		query = query.Where("name ILIKE ? OR code ILIKE ? OR module ILIKE ?", like, like, like)
	}
	if f.Module != "" {
		query = query.Where("module = ?", f.Module)
	}
	if f.Name != "" {
		query = query.Where("name ILIKE ?", "%"+f.Name+"%")
	}
	if f.Code != "" {
		query = query.Where("code ILIKE ?", "%"+f.Code+"%")
	}

	var perms []model.Permission
	err := query.Order("module, resource, action").Find(&perms).Error
	return perms, err
}

// ListByModule 按模块获取权限
func (r *PermissionRepository) ListByModule(ctx context.Context, module string) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.WithContext(ctx).Where("module = ?", module).Order("resource, action").Find(&perms).Error
	return perms, err
}

// GetUserPermissions 获取用户的所有权限
func (r *PermissionRepository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.WithContext(ctx).
		Distinct("permissions.*").
		Table("permissions").
		Joins("INNER JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("INNER JOIN user_roles ON user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ?", userID).
		Find(&perms).Error
	return perms, err
}

// GetPermissionCodes 获取用户的权限码列表
func (r *PermissionRepository) GetPermissionCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	perms, err := r.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	codes := make([]string, len(perms))
	for i, p := range perms {
		codes[i] = p.Code
	}
	return codes, nil
}
