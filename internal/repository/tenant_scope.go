package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DefaultTenantID 默认租户ID
var DefaultTenantID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// tenantIDKey 上下文中存储 tenantID 的 key
type tenantIDKey struct{}

// WithTenantID 将租户 ID 注入到 context 中
func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, tenantID)
}

// TenantIDFromContext 从 context 中获取租户 ID
// 如果不存在，返回 DefaultTenantID
func TenantIDFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(tenantIDKey{}).(uuid.UUID); ok {
		return id
	}
	return DefaultTenantID
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
	tenantID := TenantIDFromContext(ctx)
	return db.WithContext(ctx).Where("tenant_id = ?", tenantID)
}

// isUniqueViolation 检查错误是否为 PostgreSQL 唯一约束冲突（SQLSTATE 23505）
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key")
}
