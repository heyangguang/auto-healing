package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListByRequester 查询指定平台管理员的申请列表
func (r *ImpersonationRepository) ListByRequester(ctx context.Context, requesterID uuid.UUID, status string, tenantName, reason query.StringFilter, page, pageSize int) ([]model.ImpersonationRequest, int64, error) {
	var requests []model.ImpersonationRequest
	var total int64
	q := r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).Where("requester_id = ?", requesterID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q = query.ApplyStringFilter(q, "tenant_name", tenantName)
	q = query.ApplyStringFilter(q, "reason", reason)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&requests).Error
	if err != nil {
		return nil, 0, err
	}
	if err := r.fillApproverNames(ctx, requests); err != nil {
		return nil, 0, err
	}
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

// ListByTenant 查询指定租户的所有审批记录
func (r *ImpersonationRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int, filters map[string]string) ([]model.ImpersonationRequest, int64, error) {
	var requests []model.ImpersonationRequest
	var total int64
	queryBuilder := r.db.WithContext(ctx).Model(&model.ImpersonationRequest{}).Where("tenant_id = ?", tenantID)
	if value := filters["requester_name"]; value != "" {
		queryBuilder = queryBuilder.Where("requester_name ILIKE ?", "%"+value+"%")
	}
	if value := filters["reason"]; value != "" {
		queryBuilder = queryBuilder.Where("reason ILIKE ?", "%"+value+"%")
	}
	if value := filters["status"]; value != "" {
		queryBuilder = queryBuilder.Where("status = ?", value)
	}
	if err := queryBuilder.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := queryBuilder.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&requests).Error
	if err != nil {
		return nil, 0, err
	}
	if err := r.fillApproverNames(ctx, requests); err != nil {
		return nil, 0, err
	}
	return requests, total, nil
}

func (r *ImpersonationRepository) fillApproverNames(ctx context.Context, requests []model.ImpersonationRequest) error {
	approverIDs := make([]uuid.UUID, 0)
	for _, req := range requests {
		if req.ApprovedBy != nil {
			approverIDs = append(approverIDs, *req.ApprovedBy)
		}
	}
	if len(approverIDs) == 0 {
		return nil
	}

	var users []struct {
		ID       uuid.UUID `gorm:"column:id"`
		Username string    `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).
		Table("users").
		Select("id, username").
		Where("id IN ?", approverIDs).
		Scan(&users).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	nameMap := make(map[uuid.UUID]string, len(users))
	for _, user := range users {
		nameMap[user.ID] = user.Username
	}
	for i := range requests {
		if requests[i].ApprovedBy != nil {
			requests[i].ApproverName = nameMap[*requests[i].ApprovedBy]
		}
	}
	return nil
}
