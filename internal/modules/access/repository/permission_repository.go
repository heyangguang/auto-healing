package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PermissionRepository 权限数据仓库
type PermissionRepository struct {
	db *gorm.DB
}

const platformPermissionCodePrefix = "platform:"

var ErrTenantPermissionScope = errors.New("租户角色只能分配租户权限")

// PermissionFilter 权限过滤参数
type PermissionFilter struct {
	Module string
	Name   string
	Code   string
}

// NewPermissionRepository 创建权限仓库
func NewPermissionRepository() *PermissionRepository {
	return &PermissionRepository{db: database.DB}
}

func NewPermissionRepositoryWithDB(db *gorm.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// GetByID 根据 ID 获取权限
func (r *PermissionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Permission, error) {
	return r.getPermission(ctx, "id = ?", id)
}

// GetByCode 根据权限码获取权限
func (r *PermissionRepository) GetByCode(ctx context.Context, code string) (*model.Permission, error) {
	return r.getPermission(ctx, "code = ?", code)
}

func (r *PermissionRepository) getPermission(ctx context.Context, predicate string, value any) (*model.Permission, error) {
	var perm model.Permission
	err := r.db.WithContext(ctx).First(&perm, predicate, value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPermissionNotFound
	}
	return &perm, err
}

// List 获取所有权限
func (r *PermissionRepository) List(ctx context.Context) ([]model.Permission, error) {
	return r.listPermissions(ctx, PermissionFilter{}, false)
}

// ListTenant 获取租户可分配的权限
func (r *PermissionRepository) ListTenant(ctx context.Context) ([]model.Permission, error) {
	return r.listPermissions(ctx, PermissionFilter{}, true)
}

func (r *PermissionRepository) listPermissions(ctx context.Context, f PermissionFilter, tenantOnly bool) ([]model.Permission, error) {
	var perms []model.Permission
	queryBuilder := buildPermissionFilterQuery(r.db.WithContext(ctx), f)
	if tenantOnly {
		queryBuilder = applyTenantPermissionScope(queryBuilder)
	}
	err := queryBuilder.Order("module, resource, action").Find(&perms).Error
	return perms, err
}

func (r *PermissionRepository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Permission{}).Count(&count).Error
	return count, err
}

// ListWithFilter 带过滤条件获取权限
func (r *PermissionRepository) ListWithFilter(ctx context.Context, f PermissionFilter) ([]model.Permission, error) {
	return r.listPermissions(ctx, f, false)
}

// ListTenantWithFilter 带过滤条件获取租户可分配权限
func (r *PermissionRepository) ListTenantWithFilter(ctx context.Context, f PermissionFilter) ([]model.Permission, error) {
	return r.listPermissions(ctx, f, true)
}

// ListByModule 按模块获取权限
func (r *PermissionRepository) ListByModule(ctx context.Context, module string) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.WithContext(ctx).Where("module = ?", module).Order("resource, action").Find(&perms).Error
	return perms, err
}

// GetUserPermissions 获取用户的所有权限（合并平台角色 + 租户角色）
func (r *PermissionRepository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.WithContext(ctx).
		Distinct("permissions.*").
		Table("permissions").
		Joins("INNER JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where(`role_permissions.role_id IN (
			SELECT role_id FROM user_platform_roles WHERE user_id = ?
			UNION
			SELECT user_tenant_roles.role_id
			FROM user_tenant_roles
			INNER JOIN tenants ON tenants.id = user_tenant_roles.tenant_id
			WHERE user_tenant_roles.user_id = ? AND tenants.status = ?
		)`, userID, userID, model.TenantStatusActive).
		Find(&perms).Error
	return perms, err
}

// GetPermissionCodes 获取用户的权限码列表（所有租户合并，用于向后兼容）
func (r *PermissionRepository) GetPermissionCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	perms, err := r.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	return permissionCodes(perms), nil
}

// GetTenantPermissionCodes 获取用户在指定租户下的权限码列表
func (r *PermissionRepository) GetTenantPermissionCodes(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) ([]string, error) {
	var perms []model.Permission
	err := r.db.WithContext(ctx).
		Distinct("permissions.*").
		Table("permissions").
		Joins("INNER JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("permissions.code NOT LIKE ?", platformPermissionCodePrefix+"%").
		Where(`role_permissions.role_id IN (
			SELECT role_id FROM user_platform_roles WHERE user_id = ?
			UNION
			SELECT role_id FROM user_tenant_roles WHERE user_id = ? AND tenant_id = ?
		)`, userID, userID, tenantID).
		Find(&perms).Error
	if err != nil {
		return nil, err
	}
	return permissionCodes(perms), nil
}

// GetPlatformPermissionCodes 获取用户的平台角色权限码列表
func (r *PermissionRepository) GetPlatformPermissionCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var perms []model.Permission
	err := r.db.WithContext(ctx).
		Distinct("permissions.*").
		Table("permissions").
		Joins("INNER JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where(`role_permissions.role_id IN (
			SELECT role_id FROM user_platform_roles WHERE user_id = ?
		)`, userID).
		Find(&perms).Error
	if err != nil {
		return nil, err
	}
	return permissionCodes(perms), nil
}

func (r *PermissionRepository) ValidateTenantPermissionIDs(ctx context.Context, permissionIDs []uuid.UUID) error {
	uniqueIDs := uniquePermissionIDs(permissionIDs)
	if len(uniqueIDs) == 0 {
		return nil
	}

	var count int64
	err := applyTenantPermissionScope(r.db.WithContext(ctx).Model(&model.Permission{})).
		Where("id IN ?", uniqueIDs).
		Distinct("id").
		Count(&count).Error
	if err != nil {
		return err
	}
	if count != int64(len(uniqueIDs)) {
		return ErrTenantPermissionScope
	}
	return nil
}

func buildPermissionFilterQuery(queryBuilder *gorm.DB, f PermissionFilter) *gorm.DB {
	if f.Module != "" {
		queryBuilder = queryBuilder.Where("module = ?", f.Module)
	}
	if f.Name != "" {
		queryBuilder = queryBuilder.Where("name ILIKE ?", "%"+f.Name+"%")
	}
	if f.Code != "" {
		queryBuilder = queryBuilder.Where("code ILIKE ?", "%"+f.Code+"%")
	}
	return queryBuilder
}

func applyTenantPermissionScope(queryBuilder *gorm.DB) *gorm.DB {
	return queryBuilder.Where("code NOT LIKE ?", platformPermissionCodePrefix+"%")
}

func uniquePermissionIDs(permissionIDs []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(permissionIDs))
	uniqueIDs := make([]uuid.UUID, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		if _, ok := seen[permissionID]; ok {
			continue
		}
		seen[permissionID] = struct{}{}
		uniqueIDs = append(uniqueIDs, permissionID)
	}
	return uniqueIDs
}

func permissionCodes(perms []model.Permission) []string {
	codes := make([]string, len(perms))
	for i, perm := range perms {
		codes[i] = perm.Code
	}
	return codes
}
