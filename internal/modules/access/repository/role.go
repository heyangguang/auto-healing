package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/access/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RoleRepository 角色数据仓库
type RoleRepository struct {
	db *gorm.DB
}

// RoleFilter 角色过滤参数
type RoleFilter struct {
	Name     string
	Scope    string
	TenantID uuid.UUID
}

// RoleStats 角色统计信息
type RoleStats struct {
	RoleID          string `json:"role_id"`
	UserCount       int64  `json:"user_count"`
	PermissionCount int64  `json:"permission_count"`
}

// RoleUserInfo 角色关联用户信息
type RoleUserInfo struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	Status      string    `json:"status"`
}

// NewRoleRepository 创建角色仓库
func NewRoleRepository() *RoleRepository {
	return &RoleRepository{db: database.DB}
}

func NewRoleRepositoryWithDB(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// Create 创建角色
func (r *RoleRepository) Create(ctx context.Context, role *model.Role) error {
	if role.Scope == "tenant" {
		if err := platformrepo.FillTenantID(ctx, &role.TenantID); err != nil {
			return err
		}
	}
	return r.db.WithContext(ctx).Create(role).Error
}

// GetByID 根据 ID 获取角色
func (r *RoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Role, error) {
	return r.getRole(ctx, "id = ?", id)
}

// GetByName 根据名称获取角色
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*model.Role, error) {
	return r.getRole(ctx, "name = ?", name)
}

func (r *RoleRepository) getRole(ctx context.Context, predicate string, value any) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).Preload("Permissions").First(&role, predicate, value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}
	return &role, err
}

// Update 更新角色
func (r *RoleRepository) Update(ctx context.Context, role *model.Role) error {
	if role.Scope == "tenant" || role.TenantID != nil {
		return platformrepo.UpdateTenantScopedModel(r.db, ctx, role.ID, role)
	}
	return r.db.WithContext(ctx).Save(role).Error
}

// Delete 删除角色
func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	role, err := r.loadRoleForDelete(ctx, id)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return errors.New("不能删除系统内置角色")
	}
	return r.db.WithContext(ctx).Delete(&model.Role{}, "id = ?", id).Error
}

func (r *RoleRepository) loadRoleForDelete(ctx context.Context, id uuid.UUID) (*model.Role, error) {
	if tenantID, ok := platformrepo.TenantIDFromContextOK(ctx); ok {
		return r.GetTenantRoleByID(ctx, tenantID, id)
	}
	var role model.Role
	if err := r.db.WithContext(ctx).First(&role, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// List 获取角色列表
func (r *RoleRepository) List(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).Preload("Permissions").Order("is_system DESC, created_at ASC").Find(&roles).Error
	return roles, err
}
