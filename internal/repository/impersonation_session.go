package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UpdateStatus 更新申请状态
func (r *ImpersonationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, approvedBy *uuid.UUID) error {
	updates := map[string]interface{}{"status": status, "updated_at": time.Now()}
	if approvedBy != nil {
		now := time.Now()
		updates["approved_by"] = approvedBy
		updates["approved_at"] = &now
	}
	if status == model.ImpersonationStatusRejected {
		now := time.Now()
		updates["completed_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).Where("id = ?", id).Updates(updates).Error
}

// StartSession 开始 Impersonation 会话
func (r *ImpersonationRepository) StartSession(ctx context.Context, id uuid.UUID, durationMinutes int) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":             model.ImpersonationStatusActive,
		"session_started_at": now,
		"session_expires_at": now.Add(time.Duration(durationMinutes) * time.Minute),
		"updated_at":         now,
	}).Error
}

// CompleteSession 结束 Impersonation 会话
func (r *ImpersonationRepository) CompleteSession(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       model.ImpersonationStatusCompleted,
		"completed_at": now,
		"updated_at":   now,
	}).Error
}

// PauseSession 暂离 Impersonation 会话
func (r *ImpersonationRepository) PauseSession(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     model.ImpersonationStatusApproved,
		"updated_at": time.Now(),
	}).Error
}

// ResumeSession 恢复暂离的 Impersonation 会话
func (r *ImpersonationRepository) ResumeSession(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).Where("id = ?", id).Updates(map[string]interface{}{
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

// ExpireOverdueSessions 批量过期超时的会话
func (r *ImpersonationRepository) ExpireOverdueSessions(ctx context.Context) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).
		Where("status IN (?, ?) AND session_expires_at IS NOT NULL AND session_expires_at < ?",
			model.ImpersonationStatusActive, model.ImpersonationStatusApproved, now).
		Updates(map[string]interface{}{"status": model.ImpersonationStatusExpired, "completed_at": now, "updated_at": now})
	return result.RowsAffected, result.Error
}

// CancelRequest 撤销申请（仅 pending 状态可撤销）
func (r *ImpersonationRepository) CancelRequest(ctx context.Context, id, requesterID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).
		Where("id = ? AND requester_id = ? AND status = ?", id, requesterID, model.ImpersonationStatusPending).
		Updates(map[string]interface{}{"status": model.ImpersonationStatusCancelled, "updated_at": time.Now()})
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}
