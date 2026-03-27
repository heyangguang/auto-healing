package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== 邀请管理 Repository ====================

// InvitationRepository 邀请仓库
type InvitationRepository struct {
	db *gorm.DB
}

// NewInvitationRepository 创建邀请仓库
func NewInvitationRepository() *InvitationRepository {
	return NewInvitationRepositoryWithDB(database.DB)
}

func NewInvitationRepositoryWithDB(db *gorm.DB) *InvitationRepository {
	return &InvitationRepository{db: db}
}

// Create 创建邀请记录
func (r *InvitationRepository) Create(ctx context.Context, inv *model.TenantInvitation) error {
	return r.db.WithContext(ctx).Create(inv).Error
}

// GetByID 按 ID 获取邀请（手动加载关联）
func (r *InvitationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.TenantInvitation, error) {
	var inv model.TenantInvitation
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&inv).Error
	if err != nil {
		return nil, err
	}
	r.loadAssociations(ctx, &inv)
	return &inv, nil
}

// GetByTokenHash 通过 token 哈希查找有效邀请
func (r *InvitationRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.TenantInvitation, error) {
	var inv model.TenantInvitation
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND status = ?", tokenHash, model.InvitationStatusPending).
		First(&inv).Error
	if err != nil {
		return nil, err
	}
	r.loadAssociations(ctx, &inv)
	return &inv, nil
}

// ListByTenant 列出租户的邀请（分页）
func (r *InvitationRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]model.TenantInvitation, int64, error) {
	var invitations []model.TenantInvitation
	var total int64

	query := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Model(&model.TenantInvitation{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&invitations).Error
	if err != nil {
		return nil, 0, err
	}

	// 手动加载关联
	for i := range invitations {
		r.loadAssociations(ctx, &invitations[i])
	}

	return invitations, total, nil
}

// UpdateStatus 更新邀请状态
func (r *InvitationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if status == model.InvitationStatusAccepted {
		now := time.Now()
		updates["accepted_at"] = now
	}
	return r.db.WithContext(ctx).Model(&model.TenantInvitation{}).
		Where("id = ?", id).Updates(updates).Error
}

// Delete 删除邀请
func (r *InvitationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.TenantInvitation{}, "id = ?", id).Error
}

// CheckEmailPendingInTenant 检查邮箱在租户内是否有待处理邀请
func (r *InvitationRepository) CheckEmailPendingInTenant(ctx context.Context, tenantID uuid.UUID, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.TenantInvitation{}).
		Where("tenant_id = ? AND email = ? AND status = ?", tenantID, email, model.InvitationStatusPending).
		Count(&count).Error
	return count > 0, err
}

// ExpireOldInvitations 过期旧邀请
func (r *InvitationRepository) ExpireOldInvitations(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.TenantInvitation{}).
		Where("status = ? AND expires_at < ?", model.InvitationStatusPending, time.Now()).
		Updates(map[string]interface{}{
			"status":     model.InvitationStatusExpired,
			"updated_at": time.Now(),
		})
	return result.RowsAffected, result.Error
}

// loadAssociations 手动加载关联数据（因为 gorm:"-" 字段不能用 Preload）
func (r *InvitationRepository) loadAssociations(ctx context.Context, inv *model.TenantInvitation) {
	// 加载租户
	var tenant model.Tenant
	if err := r.db.WithContext(ctx).Where("id = ?", inv.TenantID).First(&tenant).Error; err == nil {
		inv.Tenant = &tenant
	}
	// 加载角色
	var role model.Role
	if err := r.db.WithContext(ctx).Where("id = ?", inv.RoleID).First(&role).Error; err == nil {
		inv.Role = &role
	}
	// 加载邀请人
	var inviter model.User
	if err := r.db.WithContext(ctx).Select("id, username, display_name, email").Where("id = ?", inv.InvitedBy).First(&inviter).Error; err == nil {
		inv.Inviter = &inviter
	}
}
