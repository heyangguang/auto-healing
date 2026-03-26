package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLogListOptions 审计日志查询选项
type AuditLogListOptions struct {
	Page                 int
	PageSize             int
	Search               query.StringFilter
	Category             string
	Action               string
	ResourceType         string
	ExcludeActions       []string
	ExcludeResourceTypes []string
	Username             query.StringFilter
	UserID               *uuid.UUID
	Status               string
	RiskLevel            string
	RequestPath          query.StringFilter
	CreatedAfter         *time.Time
	CreatedBefore        *time.Time
	SortBy               string
	SortOrder            string
}

// AuditLogRepository 审计日志仓库
type AuditLogRepository struct {
	db *gorm.DB
}

// NewAuditLogRepository 创建审计日志仓库
func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

func (r *AuditLogRepository) tenantDB(ctx context.Context) *gorm.DB {
	return TenantDB(r.db, ctx)
}

// Create 创建审计日志
func (r *AuditLogRepository) Create(ctx context.Context, log *model.AuditLog) error {
	if log.TenantID == nil {
		tenantID, ok := TenantIDFromContextOK(ctx)
		if !ok {
			return r.db.WithContext(ctx).Create(log).Error
		}
		log.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByID 根据 ID 获取审计日志
func (r *AuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AuditLog, error) {
	var log model.AuditLog
	err := r.tenantDB(ctx).Preload("User").First(&log, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}
