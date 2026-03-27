package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateTemplate 创建通知模板
func (r *NotificationRepository) CreateTemplate(ctx context.Context, template *model.NotificationTemplate) error {
	if err := FillTenantID(ctx, &template.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(template).Error
}

// GetTemplateByID 根据 ID 获取模板
func (r *NotificationRepository) GetTemplateByID(ctx context.Context, id uuid.UUID) (*model.NotificationTemplate, error) {
	var template model.NotificationTemplate
	if err := r.tenantDB(ctx).Where("id = ?", id).First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

// GetTemplatesByIDs 批量获取模板
func (r *NotificationRepository) GetTemplatesByIDs(ctx context.Context, ids []uuid.UUID) ([]model.NotificationTemplate, error) {
	var templates []model.NotificationTemplate
	if err := r.tenantDB(ctx).Where("id IN ?", ids).Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// ListTemplates 获取模板列表
func (r *NotificationRepository) ListTemplates(ctx context.Context, opts *TemplateListOptions) ([]model.NotificationTemplate, int64, error) {
	var templates []model.NotificationTemplate
	queryBuilder := r.applyTemplateFilters(r.tenantDB(ctx).Model(&model.NotificationTemplate{}), opts)
	total, err := countWithSession(queryBuilder)
	if err != nil {
		return nil, 0, err
	}

	offset := (opts.Page - 1) * opts.PageSize
	err = queryBuilder.Offset(offset).Limit(opts.PageSize).Order(templateOrderClause(opts)).Find(&templates).Error
	if err != nil {
		return nil, 0, err
	}
	return templates, total, nil
}

// UpdateTemplate 更新模板
func (r *NotificationRepository) UpdateTemplate(ctx context.Context, template *model.NotificationTemplate) error {
	return UpdateTenantScopedModel(r.db, ctx, template.ID, template)
}

// DeleteTemplate 删除模板
func (r *NotificationRepository) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	return r.tenantDB(ctx).Delete(&model.NotificationTemplate{}, "id = ?", id).Error
}

func (r *NotificationRepository) applyTemplateFilters(db *gorm.DB, opts *TemplateListOptions) *gorm.DB {
	if !opts.Name.IsEmpty() {
		db = query.ApplyStringFilter(db, "name", opts.Name)
	}
	if opts.EventType != "" {
		db = db.Where("event_type = ?", opts.EventType)
	}
	if opts.IsActive != nil {
		db = db.Where("is_active = ?", *opts.IsActive)
	}
	if opts.Format != "" {
		db = db.Where("format = ?", opts.Format)
	}
	if opts.SupportedChannel != "" {
		db = db.Where("supported_channels @> ?", `["`+opts.SupportedChannel+`"]`)
	}
	return db
}

func templateOrderClause(opts *TemplateListOptions) string {
	orderClause := "created_at DESC"
	if opts.SortBy == "" {
		return orderClause
	}

	order := "ASC"
	if opts.SortOrder == "desc" {
		order = "DESC"
	}
	allowedSortFields := map[string]bool{
		"name": true, "created_at": true, "updated_at": true, "format": true, "event_type": true,
	}
	if allowedSortFields[opts.SortBy] {
		orderClause = opts.SortBy + " " + order
	}
	return orderClause
}
