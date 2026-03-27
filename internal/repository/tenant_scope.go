package repository

import (
	"context"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrTenantContextRequired = platformrepo.ErrTenantContextRequired

// WithTenantID 将租户 ID 注入到 context 中
func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return platformrepo.WithTenantID(ctx, tenantID)
}

// TenantIDFromContext 从 context 中获取租户 ID。
// 如果不存在，返回 uuid.Nil；tenant-only 路径应改用 RequireTenantID。
func TenantIDFromContext(ctx context.Context) uuid.UUID {
	return platformrepo.TenantIDFromContext(ctx)
}

// TenantIDFromContextOK 从 context 中获取租户 ID，并返回是否显式设置
func TenantIDFromContextOK(ctx context.Context) (uuid.UUID, bool) {
	return platformrepo.TenantIDFromContextOK(ctx)
}

func RequireTenantID(ctx context.Context) (uuid.UUID, error) {
	return platformrepo.RequireTenantID(ctx)
}

func FillTenantID(ctx context.Context, tenantID **uuid.UUID) error {
	return platformrepo.FillTenantID(ctx, tenantID)
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
	return platformrepo.TenantDB(db, ctx)
}

func UpdateTenantScopedModel(db *gorm.DB, ctx context.Context, id uuid.UUID, entity interface{}, omit ...string) error {
	return platformrepo.UpdateTenantScopedModel(db, ctx, id, entity, omit...)
}
