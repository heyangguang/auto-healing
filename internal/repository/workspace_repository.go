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

// ==================== 系统工作区 CRUD ====================

// Create 创建系统工作区
func (r *WorkspaceRepository) Create(ctx context.Context, ws *model.SystemWorkspace) error {
	// 自动设置 tenant_id（List/GetByID 使用 TenantDB 过滤）
	if ws.TenantID == nil {
		tid := TenantIDFromContext(ctx)
		ws.TenantID = &tid
	}
	return r.db.WithContext(ctx).Create(ws).Error
}

// CreateAndAssignToRoles 创建工作区并在同一事务中分配给指定角色。
func (r *WorkspaceRepository) CreateAndAssignToRoles(ctx context.Context, ws *model.SystemWorkspace, roleIDs []uuid.UUID) error {
	tenantID := TenantIDFromContext(ctx)
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
	return r.db.WithContext(ctx).Save(ws).Error
}

// Delete 删除系统工作区（级联删除关联）
func (r *WorkspaceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Delete(&model.SystemWorkspace{}, "id = ?", id).Error
}

// ==================== 角色-工作区关联 ====================

// AssignToRole 为角色分配工作区（先删后建）
func (r *WorkspaceRepository) AssignToRole(ctx context.Context, roleID uuid.UUID, workspaceIDs []uuid.UUID) error {
	tenantID := TenantIDFromContext(ctx)
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
	tenantID := TenantIDFromContext(ctx)
	err := r.db.WithContext(ctx).Where("role_id = ? AND tenant_id = ?", roleID, tenantID).Find(&rws).Error
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
	// 查询角色关联的工作区 OR 默认工作区（is_default=true 对所有用户可见）
	tenantID := TenantIDFromContext(ctx)
	// 合并 user_platform_roles + user_tenant_roles 的角色
	err := r.db.WithContext(ctx).
		Raw(`SELECT DISTINCT sw.* FROM system_workspaces sw
			LEFT JOIN role_workspaces rw ON rw.workspace_id = sw.id AND rw.tenant_id = ?
			WHERE (
				rw.role_id IN (
					SELECT role_id FROM user_platform_roles WHERE user_id = ?
					UNION
					SELECT role_id FROM user_tenant_roles WHERE user_id = ? AND tenant_id = ?
				)
				OR sw.is_default = true
			) AND sw.tenant_id = ?
			ORDER BY sw.is_default DESC, sw.created_at ASC`, tenantID, userID, userID, tenantID, tenantID).
		Scan(&workspaces).Error
	return workspaces, err
}

// GetUserRoleIDs 获取用户的所有角色 ID（合并平台角色 + 租户角色）
func (r *WorkspaceRepository) GetUserRoleIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var roleIDs []uuid.UUID
	// 合并 user_platform_roles + user_tenant_roles
	tenantID := TenantIDFromContext(ctx)
	err := r.db.WithContext(ctx).
		Raw(`SELECT role_id FROM user_platform_roles WHERE user_id = ?
			UNION
			SELECT role_id FROM user_tenant_roles WHERE user_id = ? AND tenant_id = ?`, userID, userID, tenantID).
		Pluck("role_id", &roleIDs).Error
	return roleIDs, err
}

// GetRoleExplicitWorkspaceIDs 获取角色在 role_workspaces 表中明确分配的工作区 ID（不含自动追加的默认工作区）
func (r *WorkspaceRepository) GetRoleExplicitWorkspaceIDs(ctx context.Context, roleID uuid.UUID) ([]uuid.UUID, error) {
	var rws []model.RoleWorkspace
	tenantID := TenantIDFromContext(ctx)
	err := r.db.WithContext(ctx).Where("role_id = ? AND tenant_id = ?", roleID, tenantID).Find(&rws).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(rws))
	for i, rw := range rws {
		ids[i] = rw.WorkspaceID
	}
	return ids, err
}
