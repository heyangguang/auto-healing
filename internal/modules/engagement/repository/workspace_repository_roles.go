package repository

import (
	"context"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AssignToRole 为角色分配工作区（先删后建，默认工作区由后端自动生效）
func (r *WorkspaceRepository) AssignToRole(ctx context.Context, roleID uuid.UUID, workspaceIDs []uuid.UUID) error {
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return err
	}
	explicitIDs, err := r.validateRoleWorkspaceIDs(ctx, tenantID, workspaceIDs)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ? AND tenant_id = ?", roleID, tenantID).Delete(&model.RoleWorkspace{}).Error; err != nil {
			return err
		}
		for _, wsID := range explicitIDs {
			rw := model.RoleWorkspace{
				ID:          uuid.New(),
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
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Where("role_id = ? AND tenant_id = ?", roleID, tenantID).Find(&rws).Error; err != nil {
		return nil, err
	}
	idSet := make(map[uuid.UUID]bool)
	ids := make([]uuid.UUID, 0, len(rws))
	for _, rw := range rws {
		if idSet[rw.WorkspaceID] {
			continue
		}
		idSet[rw.WorkspaceID] = true
		ids = append(ids, rw.WorkspaceID)
	}

	var defaultWs []model.SystemWorkspace
	if err := platformrepo.TenantDB(r.db, ctx).Where("is_default = ?", true).Select("id").Find(&defaultWs).Error; err != nil {
		return nil, err
	}
	for _, ws := range defaultWs {
		if idSet[ws.ID] {
			continue
		}
		idSet[ws.ID] = true
		ids = append(ids, ws.ID)
	}
	return ids, nil
}

// GetWorkspacesByUserRoles 获取用户所有角色关联的系统工作区 + 默认工作区（去重）
func (r *WorkspaceRepository) GetWorkspacesByUserRoles(ctx context.Context, userID uuid.UUID) ([]model.SystemWorkspace, error) {
	var workspaces []model.SystemWorkspace
	tenantID, err := platformrepo.RequireTenantID(ctx)
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
	tenantID, err := platformrepo.RequireTenantID(ctx)
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

// GetRoleExplicitWorkspaceIDs 获取角色在 role_workspaces 表中明确分配的工作区 ID（不含自动追加的默认工作区）
func (r *WorkspaceRepository) GetRoleExplicitWorkspaceIDs(ctx context.Context, roleID uuid.UUID) ([]uuid.UUID, error) {
	var rws []model.RoleWorkspace
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Where("role_id = ? AND tenant_id = ?", roleID, tenantID).Find(&rws).Error; err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(rws))
	for i, rw := range rws {
		ids[i] = rw.WorkspaceID
	}
	return ids, nil
}

func (r *WorkspaceRepository) validateRoleWorkspaceIDs(ctx context.Context, tenantID uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error) {
	uniqueIDs := dedupeWorkspaceIDs(workspaceIDs)
	if len(uniqueIDs) == 0 {
		return nil, nil
	}

	var workspaces []model.SystemWorkspace
	if err := r.db.WithContext(ctx).
		Select("id", "is_default").
		Where("tenant_id = ? AND id IN ?", tenantID, uniqueIDs).
		Find(&workspaces).Error; err != nil {
		return nil, err
	}
	found := make(map[uuid.UUID]model.SystemWorkspace, len(workspaces))
	for _, ws := range workspaces {
		found[ws.ID] = ws
	}

	explicitIDs := make([]uuid.UUID, 0, len(uniqueIDs))
	missingIDs := make([]uuid.UUID, 0)
	for _, id := range uniqueIDs {
		ws, ok := found[id]
		if !ok {
			missingIDs = append(missingIDs, id)
			continue
		}
		if ws.IsDefault {
			continue
		}
		explicitIDs = append(explicitIDs, id)
	}
	if len(missingIDs) > 0 {
		return nil, &WorkspaceScopeError{IDs: missingIDs}
	}
	return explicitIDs, nil
}

func dedupeWorkspaceIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	uniqueIDs := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		uniqueIDs = append(uniqueIDs, id)
	}
	return uniqueIDs
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
