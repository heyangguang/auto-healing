package repositoryx

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrTenantContextRequired = errors.New("tenant context is required")

type tenantIDKey struct{}

func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, tenantID)
}

func TenantIDFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(tenantIDKey{}).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

func TenantIDFromContextOK(ctx context.Context) (uuid.UUID, bool) {
	if id, ok := ctx.Value(tenantIDKey{}).(uuid.UUID); ok {
		return id, true
	}
	return uuid.Nil, false
}

func RequireTenantID(ctx context.Context) (uuid.UUID, error) {
	if id, ok := TenantIDFromContextOK(ctx); ok {
		return id, nil
	}
	return uuid.Nil, ErrTenantContextRequired
}

func FillTenantID(ctx context.Context, tenantID **uuid.UUID) error {
	if *tenantID != nil {
		return nil
	}
	id, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	*tenantID = &id
	return nil
}

func TenantDB(db *gorm.DB, ctx context.Context) *gorm.DB {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		scoped := db.WithContext(ctx)
		scoped.AddError(err)
		return scoped
	}
	return db.WithContext(ctx).Where("tenant_id = ?", tenantID)
}

func UpdateTenantScopedModel(db *gorm.DB, ctx context.Context, id uuid.UUID, value any, omit ...string) error {
	fieldsToOmit := append([]string{"id", "created_at"}, omit...)
	return TenantDB(db, ctx).
		Model(value).
		Where("id = ?", id).
		Select("*").
		Omit(fieldsToOmit...).
		Updates(value).Error
}
