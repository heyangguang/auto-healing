package repository

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrBlacklistExemptionNotPending = errors.New("blacklist exemption is no longer pending")

type BlacklistExemptionRepository struct {
	db *gorm.DB
}

func NewBlacklistExemptionRepository(db *gorm.DB) *BlacklistExemptionRepository {
	return &BlacklistExemptionRepository{db: db}
}

// ListOptions 列表查询选项
type ExemptionListOptions struct {
	Page      int
	PageSize  int
	Status    string
	TaskID    string
	RuleID    string
	Search    string
	SortBy    string
	SortOrder string
}

// List 分页查询豁免申请
func (r *BlacklistExemptionRepository) List(ctx context.Context, opts ExemptionListOptions) ([]model.BlacklistExemption, int64, error) {
	query := TenantDB(r.db, ctx).Model(&model.BlacklistExemption{})
	now := time.Now()

	if opts.Status != "" {
		switch opts.Status {
		case "approved":
			query = query.Where("status = ? AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)", "approved")
		case "expired":
			query = query.Where("(status = ? OR (status = ? AND expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP))", "expired", "approved")
		default:
			query = query.Where("status = ?", opts.Status)
		}
	}
	if opts.TaskID != "" {
		query = query.Where("task_id = ?", opts.TaskID)
	}
	if opts.RuleID != "" {
		query = query.Where("rule_id = ?", opts.RuleID)
	}
	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where("task_name ILIKE ? OR rule_name ILIKE ? OR reason ILIKE ? OR requester_name ILIKE ?", like, like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortBy, sortOrder := normalizeExemptionSort(opts.SortBy, opts.SortOrder)

	var items []model.BlacklistExemption
	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Order(sortBy + " " + sortOrder).Offset(offset).Limit(opts.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	normalizeExemptionStatuses(items, now)
	return items, total, nil
}

// Get 获取单条
func (r *BlacklistExemptionRepository) Get(ctx context.Context, id uuid.UUID) (*model.BlacklistExemption, error) {
	var item model.BlacklistExemption
	if err := TenantDB(r.db, ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	normalizeExemptionStatus(&item, time.Now())
	return &item, nil
}

// Create 创建豁免申请
func (r *BlacklistExemptionRepository) Create(ctx context.Context, item *model.BlacklistExemption) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// UpdateStatus 更新审批状态
func (r *BlacklistExemptionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	result := TenantDB(r.db, ctx).
		Model(&model.BlacklistExemption{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrBlacklistExemptionNotPending
	}
	return nil
}

// GetApprovedByTaskID 获取任务模板的有效豁免列表（status=approved AND 未过期）
func (r *BlacklistExemptionRepository) GetApprovedByTaskID(ctx context.Context, taskID uuid.UUID) ([]model.BlacklistExemption, error) {
	var items []model.BlacklistExemption
	now := time.Now()
	err := TenantDB(r.db, ctx).
		Where("task_id = ? AND status = ? AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)", taskID, "approved").
		Find(&items).Error
	normalizeExemptionStatuses(items, now)
	return items, err
}

// ListPending 获取待审批列表（供审批中心使用）
func (r *BlacklistExemptionRepository) ListPending(ctx context.Context, opts ExemptionListOptions) ([]model.BlacklistExemption, int64, error) {
	query := TenantDB(r.db, ctx).Model(&model.BlacklistExemption{}).Where("status = ?", "pending")

	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where("task_name ILIKE ? OR rule_name ILIKE ? OR requester_name ILIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortBy, sortOrder := normalizeExemptionSort(opts.SortBy, opts.SortOrder)

	var items []model.BlacklistExemption
	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Order(sortBy + " " + sortOrder).Offset(offset).Limit(opts.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ExpireOverdue 批量过期处理
func (r *BlacklistExemptionRepository) ExpireOverdue(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.BlacklistExemption{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", "approved", time.Now()).
		Update("status", "expired")
	return result.RowsAffected, result.Error
}

func normalizeExemptionStatuses(items []model.BlacklistExemption, now time.Time) {
	for i := range items {
		normalizeExemptionStatus(&items[i], now)
	}
}

func normalizeExemptionStatus(item *model.BlacklistExemption, now time.Time) {
	if item == nil {
		return
	}
	if item.Status == "approved" && item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
		item.Status = "expired"
	}
}

func normalizeExemptionSort(sortBy, sortOrder string) (string, string) {
	allowedSortFields := map[string]bool{
		"created_at":     true,
		"updated_at":     true,
		"approved_at":    true,
		"expires_at":     true,
		"status":         true,
		"task_name":      true,
		"rule_name":      true,
		"requester_name": true,
	}
	if !allowedSortFields[sortBy] {
		sortBy = "created_at"
	}
	switch sortOrder {
	case "ASC", "asc":
		sortOrder = "ASC"
	default:
		sortOrder = "DESC"
	}
	return sortBy, sortOrder
}

// CheckDuplicate 检查是否已存在相同的待审批申请
func (r *BlacklistExemptionRepository) CheckDuplicate(ctx context.Context, taskID, ruleID uuid.UUID) (bool, error) {
	var count int64
	err := TenantDB(r.db, ctx).Model(&model.BlacklistExemption{}).
		Where("task_id = ? AND rule_id = ? AND status = ?", taskID, ruleID, "pending").
		Count(&count).Error
	return count > 0, err
}
