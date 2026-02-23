package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== Impersonation Repository ====================

// ImpersonationRepository Impersonation 数据仓库
type ImpersonationRepository struct {
	db *gorm.DB
}

// NewImpersonationRepository 创建 Impersonation 仓库
func NewImpersonationRepository() *ImpersonationRepository {
	return &ImpersonationRepository{db: database.DB}
}

// ==================== 申请管理 ====================

// Create 创建 Impersonation 申请
func (r *ImpersonationRepository) Create(ctx context.Context, req *model.ImpersonationRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

// GetByID 根据 ID 获取申请详情
func (r *ImpersonationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ImpersonationRequest, error) {
	var req model.ImpersonationRequest
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&req).Error
	if err != nil {
		return nil, err
	}

	// 填充审批人名称
	if req.ApprovedBy != nil {
		var approverName string
		r.db.WithContext(ctx).Table("users").Select("username").Where("id = ?", *req.ApprovedBy).Scan(&approverName)
		req.ApproverName = approverName
	}

	return &req, nil
}

// ListByRequester 查询指定平台管理员的申请列表（分页，支持筛选）
func (r *ImpersonationRepository) ListByRequester(ctx context.Context, requesterID uuid.UUID, status string, tenantName, reason query.StringFilter, page, pageSize int) ([]model.ImpersonationRequest, int64, error) {
	var requests []model.ImpersonationRequest
	var total int64

	q := r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).
		Where("requester_id = ?", requesterID)

	if status != "" {
		q = q.Where("status = ?", status)
	}
	q = query.ApplyStringFilter(q, "tenant_name", tenantName)
	q = query.ApplyStringFilter(q, "reason", reason)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&requests).Error
	if err != nil {
		return nil, 0, err
	}

	// 批量填充审批人名称
	r.fillApproverNames(ctx, requests)

	return requests, total, nil
}

// ListPendingByTenant 查询指定租户的待审批申请
func (r *ImpersonationRepository) ListPendingByTenant(ctx context.Context, tenantID uuid.UUID) ([]model.ImpersonationRequest, error) {
	var requests []model.ImpersonationRequest
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, model.ImpersonationStatusPending).
		Order("created_at ASC").
		Find(&requests).Error
	return requests, err
}

// ListByTenant 查询指定租户的所有审批记录（分页 + 搜索）
func (r *ImpersonationRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int, filters map[string]string) ([]model.ImpersonationRequest, int64, error) {
	var requests []model.ImpersonationRequest
	var total int64

	query := r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).
		Where("tenant_id = ?", tenantID)

	// 搜索过滤
	if v, ok := filters["requester_name"]; ok && v != "" {
		query = query.Where("requester_name ILIKE ?", "%"+v+"%")
	}
	if v, ok := filters["reason"]; ok && v != "" {
		query = query.Where("reason ILIKE ?", "%"+v+"%")
	}
	if v, ok := filters["status"]; ok && v != "" {
		query = query.Where("status = ?", v)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&requests).Error
	if err != nil {
		return nil, 0, err
	}

	// 批量填充审批人名称
	r.fillApproverNames(ctx, requests)

	return requests, total, nil
}

// UpdateStatus 更新申请状态
func (r *ImpersonationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, approvedBy *uuid.UUID) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if approvedBy != nil {
		now := time.Now()
		updates["approved_by"] = approvedBy
		updates["approved_at"] = &now
	}

	if status == model.ImpersonationStatusRejected {
		now := time.Now()
		updates["completed_at"] = &now
	}

	return r.db.WithContext(ctx).
		Model(&model.ImpersonationRequest{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// StartSession 开始 Impersonation 会话
func (r *ImpersonationRepository) StartSession(ctx context.Context, id uuid.UUID, durationMinutes int) error {
	now := time.Now()
	expiresAt := now.Add(time.Duration(durationMinutes) * time.Minute)

	return r.db.WithContext(ctx).
		Model(&model.ImpersonationRequest{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":             model.ImpersonationStatusActive,
			"session_started_at": now,
			"session_expires_at": expiresAt,
			"updated_at":         now,
		}).Error
}

// CompleteSession 结束 Impersonation 会话
func (r *ImpersonationRepository) CompleteSession(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.ImpersonationRequest{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       model.ImpersonationStatusCompleted,
			"completed_at": now,
			"updated_at":   now,
		}).Error
}

// PauseSession 暂离 Impersonation 会话
// 将状态从 active 回退到 approved，保留 session_expires_at 不变
// 允许在到期前重新进入
func (r *ImpersonationRepository) PauseSession(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.ImpersonationRequest{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     model.ImpersonationStatusApproved,
			"updated_at": time.Now(),
		}).Error
}

// ResumeSession 恢复暂离的 Impersonation 会话
// 将状态从 approved 恢复为 active（不重设过期时间）
func (r *ImpersonationRepository) ResumeSession(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.ImpersonationRequest{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     model.ImpersonationStatusActive,
			"updated_at": time.Now(),
		}).Error
}

// GetActiveSession 获取指定用户在指定租户的活跃会话
func (r *ImpersonationRepository) GetActiveSession(ctx context.Context, requesterID, tenantID uuid.UUID) (*model.ImpersonationRequest, error) {
	var req model.ImpersonationRequest
	err := r.db.WithContext(ctx).
		Where("requester_id = ? AND tenant_id = ? AND status = ? AND session_expires_at > ?",
			requesterID, tenantID, model.ImpersonationStatusActive, time.Now()).
		First(&req).Error
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// ExpireOverdueSessions 批量过期超时的会话（定时任务调用）
// 同时处理 active（进行中）和 approved（暂离/已批准未进入）状态中 session_expires_at 已过期的记录
func (r *ImpersonationRepository) ExpireOverdueSessions(ctx context.Context) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.ImpersonationRequest{}).
		Where("status IN (?, ?) AND session_expires_at IS NOT NULL AND session_expires_at < ?",
			model.ImpersonationStatusActive, model.ImpersonationStatusApproved, now).
		Updates(map[string]interface{}{
			"status":       model.ImpersonationStatusExpired,
			"completed_at": now,
			"updated_at":   now,
		})
	return result.RowsAffected, result.Error
}

// CancelRequest 撤销申请（仅 pending 状态可撤销）
func (r *ImpersonationRepository) CancelRequest(ctx context.Context, id, requesterID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.ImpersonationRequest{}).
		Where("id = ? AND requester_id = ? AND status = ?", id, requesterID, model.ImpersonationStatusPending).
		Updates(map[string]interface{}{
			"status":     model.ImpersonationStatusCancelled,
			"updated_at": time.Now(),
		})
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

// ==================== 审批人管理 ====================

// GetApprovers 获取租户的审批人列表
func (r *ImpersonationRepository) GetApprovers(ctx context.Context, tenantID uuid.UUID) ([]model.ImpersonationApprover, error) {
	var approvers []model.ImpersonationApprover
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC").
		Find(&approvers).Error
	return approvers, err
}

// SetApprovers 设置租户的审批人（全量替换）
func (r *ImpersonationRepository) SetApprovers(ctx context.Context, tenantID uuid.UUID, userIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除旧的审批人
		if err := tx.Where("tenant_id = ?", tenantID).Delete(&model.ImpersonationApprover{}).Error; err != nil {
			return err
		}

		// 插入新的审批人
		if len(userIDs) == 0 {
			return nil
		}

		approvers := make([]model.ImpersonationApprover, len(userIDs))
		for i, uid := range userIDs {
			approvers[i] = model.ImpersonationApprover{
				TenantID: tenantID,
				UserID:   uid,
			}
		}
		return tx.Create(&approvers).Error
	})
}

// IsApprover 检查用户是否是指定租户的审批人
// 如果没有配置审批人，则该租户的所有 admin 角色用户都是审批人
func (r *ImpersonationRepository) IsApprover(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	// 先检查是否有专门的审批人配置
	var approverCount int64
	if err := r.db.WithContext(ctx).
		Model(&model.ImpersonationApprover{}).
		Where("tenant_id = ?", tenantID).
		Count(&approverCount).Error; err != nil {
		return false, err
	}

	if approverCount > 0 {
		// 有配置，检查用户是否在审批人列表中
		var count int64
		err := r.db.WithContext(ctx).
			Model(&model.ImpersonationApprover{}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).
			Count(&count).Error
		return count > 0, err
	}

	// 没有配置审批人，默认该租户的所有 admin 角色用户为审批人
	var count int64
	err := r.db.WithContext(ctx).
		Table("user_tenant_roles").
		Joins("INNER JOIN roles ON roles.id = user_tenant_roles.role_id").
		Where("user_tenant_roles.tenant_id = ? AND user_tenant_roles.user_id = ? AND roles.name = ?",
			tenantID, userID, "admin").
		Count(&count).Error
	return count > 0, err
}

// ==================== 辅助方法 ====================

// fillApproverNames 批量填充审批人名称
func (r *ImpersonationRepository) fillApproverNames(ctx context.Context, requests []model.ImpersonationRequest) {
	approverIDs := make([]uuid.UUID, 0)
	for _, req := range requests {
		if req.ApprovedBy != nil {
			approverIDs = append(approverIDs, *req.ApprovedBy)
		}
	}
	if len(approverIDs) == 0 {
		return
	}

	var users []struct {
		ID       uuid.UUID `gorm:"column:id"`
		Username string    `gorm:"column:username"`
	}
	r.db.WithContext(ctx).Table("users").Select("id, username").Where("id IN ?", approverIDs).Scan(&users)

	nameMap := make(map[uuid.UUID]string)
	for _, u := range users {
		nameMap[u.ID] = u.Username
	}

	for i := range requests {
		if requests[i].ApprovedBy != nil {
			requests[i].ApproverName = nameMap[*requests[i].ApprovedBy]
		}
	}
}
