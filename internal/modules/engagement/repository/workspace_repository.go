package repository

import (
	"context"
	"errors"
	"fmt"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"strings"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrSystemWorkspaceNotFound         = errors.New("system workspace not found in current tenant")
	ErrDefaultSystemWorkspaceProtected = errors.New("default system workspace cannot be deleted")
)

type WorkspaceScopeError struct {
	IDs []uuid.UUID
}

func (e *WorkspaceScopeError) Error() string {
	return fmt.Sprintf("workspace IDs not found in current tenant: %s", joinWorkspaceIDs(e.IDs))
}

// WorkspaceRepository 系统工作区仓库
type WorkspaceRepository struct {
	db *gorm.DB
}

func NewWorkspaceRepositoryWithDB(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

// ==================== 系统工作区 CRUD ====================

// Create 创建系统工作区
func (r *WorkspaceRepository) Create(ctx context.Context, ws *model.SystemWorkspace) error {
	// 自动设置 tenant_id（List/GetByID 使用 platformrepo.TenantDB 过滤）
	if err := platformrepo.FillTenantID(ctx, &ws.TenantID); err != nil {
		return err
	}
	if ws.ID == uuid.Nil {
		ws.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(ws).Error
}

// CreateAndAssignToRoles 创建工作区并在同一事务中分配给指定角色。
func (r *WorkspaceRepository) CreateAndAssignToRoles(ctx context.Context, ws *model.SystemWorkspace, roleIDs []uuid.UUID) error {
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return err
	}
	if ws.TenantID == nil {
		ws.TenantID = &tenantID
	}
	if ws.ID == uuid.Nil {
		ws.ID = uuid.New()
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
				ID:          uuid.New(),
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
	err := platformrepo.TenantDB(r.db, ctx).Preload("Creator").First(&ws, "id = ?", id).Error
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
	err := platformrepo.TenantDB(r.db, ctx).Preload("Creator").Order("is_default DESC, created_at DESC").Find(&workspaces).Error
	return workspaces, err
}

// Update 更新系统工作区
func (r *WorkspaceRepository) Update(ctx context.Context, ws *model.SystemWorkspace) error {
	return platformrepo.UpdateTenantScopedModel(r.db, ctx, ws.ID, ws)
}

// Delete 删除系统工作区（级联删除关联）
func (r *WorkspaceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ws, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ws == nil {
		return ErrSystemWorkspaceNotFound
	}
	if ws.IsDefault {
		return ErrDefaultSystemWorkspaceProtected
	}
	return platformrepo.TenantDB(r.db, ctx).Delete(&model.SystemWorkspace{}, "id = ?", id).Error
}

func joinWorkspaceIDs(ids []uuid.UUID) string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, id.String())
	}
	return strings.Join(values, ", ")
}
