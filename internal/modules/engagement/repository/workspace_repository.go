package repository

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkspaceRepository 系统工作区仓库
type WorkspaceRepository struct {
	db *gorm.DB
}

// NewWorkspaceRepository 创建工作区仓库
func NewWorkspaceRepository() *WorkspaceRepository {
	return &WorkspaceRepository{db: database.DB}
}

func NewWorkspaceRepositoryWithDB(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

// ==================== 系统工作区 CRUD ====================

// Create 创建系统工作区
func (r *WorkspaceRepository) Create(ctx context.Context, ws *model.SystemWorkspace) error {
	// 自动设置 tenant_id（List/GetByID 使用 TenantDB 过滤）
	if err := FillTenantID(ctx, &ws.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(ws).Error
}

// CreateAndAssignToRoles 创建工作区并在同一事务中分配给指定角色。
func (r *WorkspaceRepository) CreateAndAssignToRoles(ctx context.Context, ws *model.SystemWorkspace, roleIDs []uuid.UUID) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	if ws.TenantID == nil {
		ws.TenantID = &tenantID
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ws).Error; err != nil {
			return err
		}

		for _, roleID := range roleIDs {
			var existing []uuid.UUID
			if err := tx.Model(&model.RoleWorkspace{}).
				Where("role_id = ? AND tenant_id = ?", roleID, tenantID).
				Pluck("workspace_id", &existing).Error; err != nil {
				return err
			}

			seen := make(map[uuid.UUID]bool, len(existing)+1)
			for _, id := range existing {
				seen[id] = true
			}
			if seen[ws.ID] {
				continue
			}

			rw := model.RoleWorkspace{
				TenantID:    &tenantID,
				RoleID:      roleID,
				WorkspaceID: ws.ID,
			}
			if err := tx.Create(&rw).Error; err != nil {
				return fmt.Errorf("assign workspace to role %s: %w", roleID, err)
			}
		}

		return nil
	})
}

// GetByID 根据 ID 获取系统工作区
func (r *WorkspaceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.SystemWorkspace, error) {
	var ws model.SystemWorkspace
	err := TenantDB(r.db, ctx).Preload("Creator").First(&ws, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &ws, nil
}

// List 获取所有系统工作区（包含默认工作区，默认工作区排在最前）
func (r *WorkspaceRepository) List(ctx context.Context) ([]model.SystemWorkspace, error) {
	var workspaces []model.SystemWorkspace
	err := TenantDB(r.db, ctx).Preload("Creator").Order("is_default DESC, created_at DESC").Find(&workspaces).Error
	return workspaces, err
}

// Update 更新系统工作区
func (r *WorkspaceRepository) Update(ctx context.Context, ws *model.SystemWorkspace) error {
	return UpdateTenantScopedModel(r.db, ctx, ws.ID, ws)
}

// Delete 删除系统工作区（级联删除关联）
func (r *WorkspaceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Delete(&model.SystemWorkspace{}, "id = ?", id).Error
}

// ==================== 角色-工作区关联 ====================

// AssignToRole 为角色分配工作区（先删后建）
func (r *WorkspaceRepository) AssignToRole(ctx context.Context, roleID uuid.UUID, workspaceIDs []uuid.UUID) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(workspaceIDs) > 0 {
			var count int64
			if err := tx.Model(&model.SystemWorkspace{}).
				Where("tenant_id = ? AND id IN ?", tenantID, workspaceIDs).
				Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(workspaceIDs)) {
				return fmt.Errorf("workspace list contains IDs outside the current tenant")
			}
		}

		// 删除该角色在当前租户的所有现有工作区关联
		if err := tx.Where("role_id = ? AND tenant_id = ?", roleID, tenantID).Delete(&model.RoleWorkspace{}).Error; err != nil {
			return err
		}

		// 添加新关联
		for _, wsID := range workspaceIDs {
			rw := model.RoleWorkspace{
				TenantID:    &tenantID,
				RoleID:      roleID,
				WorkspaceID: wsID,
			}
			if err := tx.Create(&rw).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetRoleWorkspaceIDs 获取角色关联的工作区 ID 列表（自动包含默认工作区）
func (r *WorkspaceRepository) GetRoleWorkspaceIDs(ctx context.Context, roleID uuid.UUID) ([]uuid.UUID, error) {
	var rws []model.RoleWorkspace
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	err = r.db.WithContext(ctx).Where("role_id = ? AND tenant_id = ?", roleID, tenantID).Find(&rws).Error
	if err != nil {
		return nil, err
	}
	idSet := make(map[uuid.UUID]bool)
	ids := make([]uuid.UUID, 0, len(rws))
	for _, rw := range rws {
		if !idSet[rw.WorkspaceID] {
			idSet[rw.WorkspaceID] = true
			ids = append(ids, rw.WorkspaceID)
		}
	}

	// 追加默认工作区 ID（对所有角色可见）
	var defaultWs []model.SystemWorkspace
	if err := TenantDB(r.db, ctx).Where("is_default = ?", true).Select("id").Find(&defaultWs).Error; err == nil {
		for _, ws := range defaultWs {
			if !idSet[ws.ID] {
				idSet[ws.ID] = true
				ids = append(ids, ws.ID)
			}
		}
	}

	return ids, nil
}

// GetWorkspacesByUserRoles 获取用户所有角色关联的系统工作区 + 默认工作区（去重）
func (r *WorkspaceRepository) GetWorkspacesByUserRoles(ctx context.Context, userID uuid.UUID) ([]model.SystemWorkspace, error) {
	var workspaces []model.SystemWorkspace
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	roleIDs, err := r.GetUserRoleIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	query := r.db.WithContext(ctx).
		Table("system_workspaces AS sw").
		Select("DISTINCT sw.*").
		Joins("LEFT JOIN role_workspaces rw ON rw.workspace_id = sw.id AND rw.tenant_id = ?", tenantID).
		Where("sw.tenant_id = ?", tenantID)
	if len(roleIDs) > 0 {
		query = query.Where("sw.is_default = ? OR rw.role_id IN ?", true, roleIDs)
	} else {
		query = query.Where("sw.is_default = ?", true)
	}
	err = query.Order("sw.is_default DESC, sw.created_at ASC").Scan(&workspaces).Error
	return workspaces, err
}

// GetUserRoleIDs 获取用户的所有角色 ID（合并平台角色 + 租户角色）
func (r *WorkspaceRepository) GetUserRoleIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	platformRoleIDs, err := pluckWorkspaceRoleIDs(r.db.WithContext(ctx).Table("user_platform_roles"), userID)
	if err != nil {
		return nil, err
	}
	tenantRoleIDs, err := pluckWorkspaceRoleIDs(r.db.WithContext(ctx).Table("user_tenant_roles").Where("tenant_id = ?", tenantID), userID)
	if err != nil {
		return nil, err
	}
	return mergeWorkspaceRoleIDs(platformRoleIDs, tenantRoleIDs), nil
}

func pluckWorkspaceRoleIDs(query *gorm.DB, userID uuid.UUID) ([]uuid.UUID, error) {
	var roleIDs []uuid.UUID
	err := query.Where("user_id = ?", userID).Pluck("role_id", &roleIDs).Error
	return roleIDs, err
}

func mergeWorkspaceRoleIDs(groups ...[]uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool)
	var merged []uuid.UUID
	for _, group := range groups {
		for _, roleID := range group {
			if seen[roleID] {
				continue
			}
			seen[roleID] = true
			merged = append(merged, roleID)
		}
	}
	return merged
}

// GetRoleExplicitWorkspaceIDs 获取角色在 role_workspaces 表中明确分配的工作区 ID（不含自动追加的默认工作区）
func (r *WorkspaceRepository) GetRoleExplicitWorkspaceIDs(ctx context.Context, roleID uuid.UUID) ([]uuid.UUID, error) {
	var rws []model.RoleWorkspace
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	err = r.db.WithContext(ctx).Where("role_id = ? AND tenant_id = ?", roleID, tenantID).Find(&rws).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(rws))
	for i, rw := range rws {
		ids[i] = rw.WorkspaceID
	}
	return ids, err
}
