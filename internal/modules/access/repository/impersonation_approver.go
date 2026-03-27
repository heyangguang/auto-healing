package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetApprovers 获取租户的审批人列表
func (r *ImpersonationRepository) GetApprovers(ctx context.Context, tenantID uuid.UUID) ([]model.ImpersonationApprover, error) {
	var approvers []model.ImpersonationApprover
	err := r.db.WithContext(ctx).Preload("User").Where("tenant_id = ?", tenantID).Order("created_at ASC").Find(&approvers).Error
	return approvers, err
}

// SetApprovers 设置租户的审批人（全量替换）
func (r *ImpersonationRepository) SetApprovers(ctx context.Context, tenantID uuid.UUID, userIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ?", tenantID).Delete(&model.ImpersonationApprover{}).Error; err != nil {
			return err
		}
		if len(userIDs) == 0 {
			return nil
		}
		approvers := make([]model.ImpersonationApprover, len(userIDs))
		for i, userID := range userIDs {
			approvers[i] = model.ImpersonationApprover{TenantID: tenantID, UserID: userID}
		}
		return tx.Create(&approvers).Error
	})
}

// IsApprover 检查用户是否是指定租户的审批人
func (r *ImpersonationRepository) IsApprover(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	var approverCount int64
	if err := r.db.WithContext(ctx).Model(&model.ImpersonationApprover{}).Where("tenant_id = ?", tenantID).Count(&approverCount).Error; err != nil {
		return false, err
	}
	if approverCount > 0 {
		var count int64
		err := r.db.WithContext(ctx).Model(&model.ImpersonationApprover{}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).
			Count(&count).Error
		return count > 0, err
	}

	var count int64
	err := r.db.WithContext(ctx).
		Table("user_tenant_roles").
		Joins("INNER JOIN roles ON roles.id = user_tenant_roles.role_id").
		Where("user_tenant_roles.tenant_id = ? AND user_tenant_roles.user_id = ? AND roles.name = ?", tenantID, userID, "admin").
		Count(&count).Error
	return count > 0, err
}
