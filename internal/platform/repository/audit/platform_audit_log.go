package audit

import (
	"context"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlatformAuditLogRepository struct {
	db *gorm.DB
}

func NewPlatformAuditLogRepository() *PlatformAuditLogRepository {
	return NewPlatformAuditLogRepositoryWithDB(database.DB)
}

func NewPlatformAuditLogRepositoryWithDB(db *gorm.DB) *PlatformAuditLogRepository {
	return &PlatformAuditLogRepository{db: db}
}

func (r *PlatformAuditLogRepository) Create(ctx context.Context, log *model.PlatformAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *PlatformAuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.PlatformAuditLog, error) {
	var log model.PlatformAuditLog
	err := r.db.WithContext(ctx).First(&log, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *PlatformAuditLogRepository) List(ctx context.Context, opts *PlatformAuditListOptions) ([]model.PlatformAuditLog, int64, error) {
	queryBuilder := applyPlatformAuditFilters(r.db.WithContext(ctx).Model(&model.PlatformAuditLog{}), opts)
	total, err := countWithClone(queryBuilder)
	if err != nil {
		return nil, 0, err
	}

	var logs []model.PlatformAuditLog
	err = queryBuilder.Order(platformAuditOrderClause(opts)).
		Offset((opts.Page - 1) * opts.PageSize).
		Limit(opts.PageSize).
		Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func applyPlatformAuditFilters(db *gorm.DB, opts *PlatformAuditListOptions) *gorm.DB {
	if opts.Category != "" {
		db = db.Where("category = ?", opts.Category)
	}
	if opts.Action != "" {
		db = db.Where("action = ?", opts.Action)
	}
	if opts.ResourceType != "" {
		db = db.Where("resource_type = ?", opts.ResourceType)
	}
	if !opts.Username.IsEmpty() {
		db = query.ApplyStringFilter(db, "username", opts.Username)
	}
	if opts.UserID != nil {
		db = db.Where("user_id = ?", *opts.UserID)
	}
	if opts.Status != "" {
		db = db.Where("status = ?", opts.Status)
	}
	if opts.CreatedAfter != nil {
		db = db.Where("created_at >= ?", *opts.CreatedAfter)
	}
	if opts.CreatedBefore != nil {
		db = db.Where("created_at <= ?", *opts.CreatedBefore)
	}
	if !opts.RequestPath.IsEmpty() {
		db = query.ApplyStringFilter(db, "request_path", opts.RequestPath)
	}
	if !opts.Search.IsEmpty() {
		db = query.ApplyMultiStringFilter(db, []string{"username", "resource_name", "request_path"}, opts.Search)
	}
	return db
}

func platformAuditOrderClause(opts *PlatformAuditListOptions) string {
	return orderClause(opts.SortBy, opts.SortOrder, map[string]bool{
		"created_at":    true,
		"action":        true,
		"resource_type": true,
		"username":      true,
		"status":        true,
		"category":      true,
	})
}
