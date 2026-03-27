package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/access/model"
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
	return NewImpersonationRepositoryWithDB(database.DB)
}

func NewImpersonationRepositoryWithDB(db *gorm.DB) *ImpersonationRepository {
	return &ImpersonationRepository{db: db}
}

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
	if req.ApprovedBy != nil {
		var approverName string
		if err := r.db.WithContext(ctx).
			Table("users").
			Select("username").
			Where("id = ?", *req.ApprovedBy).
			Scan(&approverName).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		req.ApproverName = approverName
	}
	return &req, nil
}
