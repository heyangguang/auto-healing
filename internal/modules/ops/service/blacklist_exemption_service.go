package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/ops/model"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	"github.com/google/uuid"
)

type BlacklistExemptionService struct {
	repo *opsrepo.BlacklistExemptionRepository
}

func NewBlacklistExemptionService() *BlacklistExemptionService {
	return &BlacklistExemptionService{
		repo: opsrepo.NewBlacklistExemptionRepository(database.DB),
	}
}

// List 列表查询
func (s *BlacklistExemptionService) List(ctx context.Context, opts opsrepo.ExemptionListOptions) ([]model.BlacklistExemption, int64, error) {
	return s.repo.List(ctx, opts)
}

// Get 获取单条
func (s *BlacklistExemptionService) Get(ctx context.Context, id uuid.UUID) (*model.BlacklistExemption, error) {
	return s.repo.Get(ctx, id)
}

// Create 创建豁免申请
func (s *BlacklistExemptionService) Create(ctx context.Context, item *model.BlacklistExemption) error {
	// 检查重复：同一任务+规则不能有多个 pending 申请
	dup, err := s.repo.CheckDuplicate(ctx, item.TaskID, item.RuleID)
	if err != nil {
		return err
	}
	if dup {
		return errors.New("该任务模板已有相同规则的待审批豁免申请")
	}

	item.ID = uuid.New()
	item.Status = "pending"
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	// 设置租户ID
	if err := opsrepo.FillTenantID(ctx, &item.TenantID); err != nil {
		return err
	}

	return s.repo.Create(ctx, item)
}

// Approve 审批通过
func (s *BlacklistExemptionService) Approve(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID, approverName string) error {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if item.Status != "pending" {
		return errors.New("只有待审批的申请才能审批")
	}
	// 申请人不能审批自己的申请
	if item.RequestedBy == approvedBy {
		return errors.New("申请人不能审批自己的豁免申请")
	}

	now := time.Now()
	expiresAt := now.AddDate(0, 0, item.ValidityDays)

	err = s.repo.UpdateStatus(ctx, id, map[string]interface{}{
		"status":        "approved",
		"approved_by":   approvedBy,
		"approver_name": approverName,
		"approved_at":   now,
		"expires_at":    expiresAt,
		"updated_at":    now,
	})
	return normalizeBlacklistExemptionMutationError(err)
}

// Reject 审批拒绝
func (s *BlacklistExemptionService) Reject(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID, approverName, rejectReason string) error {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if item.Status != "pending" {
		return errors.New("只有待审批的申请才能审批")
	}
	if item.RequestedBy == approvedBy {
		return errors.New("申请人不能审批自己的豁免申请")
	}

	now := time.Now()
	err = s.repo.UpdateStatus(ctx, id, map[string]interface{}{
		"status":        "rejected",
		"approved_by":   approvedBy,
		"approver_name": approverName,
		"approved_at":   now,
		"reject_reason": rejectReason,
		"updated_at":    now,
	})
	return normalizeBlacklistExemptionMutationError(err)
}

// GetApprovedByTaskID 获取任务的有效豁免规则ID列表
func (s *BlacklistExemptionService) GetApprovedByTaskID(ctx context.Context, taskID uuid.UUID) ([]model.BlacklistExemption, error) {
	return s.repo.GetApprovedByTaskID(ctx, taskID)
}

// ListPending 获取待审批列表
func (s *BlacklistExemptionService) ListPending(ctx context.Context, opts opsrepo.ExemptionListOptions) ([]model.BlacklistExemption, int64, error) {
	return s.repo.ListPending(ctx, opts)
}

// ExpireOverdue 过期处理
func (s *BlacklistExemptionService) ExpireOverdue(ctx context.Context) (int64, error) {
	return s.repo.ExpireOverdue(ctx)
}

func normalizeBlacklistExemptionMutationError(err error) error {
	if errors.Is(err, opsrepo.ErrBlacklistExemptionNotPending) {
		return fmt.Errorf("%w: 该豁免申请已被其他审批人处理", opsrepo.ErrBlacklistExemptionNotPending)
	}
	return err
}
