package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrIncidentSolutionTemplateNotFound = errors.New("工单解决方案模板不存在")

type IncidentSolutionTemplateRepository struct {
	db *gorm.DB
}

func NewIncidentSolutionTemplateRepositoryWithDB(db *gorm.DB) *IncidentSolutionTemplateRepository {
	return &IncidentSolutionTemplateRepository{db: db}
}

func (r *IncidentSolutionTemplateRepository) Create(ctx context.Context, template *model.IncidentSolutionTemplate) error {
	if err := FillTenantID(ctx, &template.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(template).Error
}

func (r *IncidentSolutionTemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.IncidentSolutionTemplate, error) {
	var template model.IncidentSolutionTemplate
	err := TenantDB(r.db, ctx).First(&template, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrIncidentSolutionTemplateNotFound
	}
	return &template, err
}

func (r *IncidentSolutionTemplateRepository) List(ctx context.Context) ([]model.IncidentSolutionTemplate, error) {
	var templates []model.IncidentSolutionTemplate
	err := TenantDB(r.db, ctx).
		Order("created_at DESC").
		Find(&templates).Error
	return templates, err
}

func (r *IncidentSolutionTemplateRepository) Update(ctx context.Context, template *model.IncidentSolutionTemplate) error {
	return UpdateTenantScopedModel(r.db, ctx, template.ID, template)
}

func (r *IncidentSolutionTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := TenantDB(r.db, ctx).Delete(&model.IncidentSolutionTemplate{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrIncidentSolutionTemplateNotFound
	}
	return nil
}
