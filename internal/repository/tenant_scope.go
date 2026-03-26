package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrTenantContextRequired = errors.New("tenant context is required")

// tenantIDKey 上下文中存储 tenantID 的 key
type tenantIDKey struct{}

// WithTenantID 将租户 ID 注入到 context 中
func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, tenantID)
}

// TenantIDFromContext 从 context 中获取租户 ID。
// 如果不存在，返回 uuid.Nil；tenant-only 路径应改用 RequireTenantID。
func TenantIDFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(tenantIDKey{}).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// TenantIDFromContextOK 从 context 中获取租户 ID，并返回是否显式设置
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

// TenantScope 租户过滤 scope，自动为查询添加 tenant_id 过滤条件
// 用法：db.Scopes(TenantScope(tenantID)).Find(&items)
func TenantScope(tenantID uuid.UUID) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("tenant_id = ?", tenantID)
	}
}

// TenantDB 从 context 读取 tenantID，返回带租户过滤的 *gorm.DB
// 这是 Repository 层的核心辅助方法：
//
//	替换 r.db.WithContext(ctx) → TenantDB(r.db, ctx)
//
// 自动应用 WHERE tenant_id = ? 条件
func TenantDB(db *gorm.DB, ctx context.Context) *gorm.DB {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		scoped := db.WithContext(ctx)
		scoped.AddError(err)
		return scoped
	}
	return db.WithContext(ctx).Where("tenant_id = ?", tenantID)
}

func UpdateTenantScopedModel(db *gorm.DB, ctx context.Context, id uuid.UUID, entity interface{}, omit ...string) error {
	fieldsToOmit := append([]string{"id", "created_at"}, omit...)
	return TenantDB(db, ctx).
		Model(entity).
		Where("id = ?", id).
		Select("*").
		Omit(fieldsToOmit...).
		Updates(entity).Error
}
