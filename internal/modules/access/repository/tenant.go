package repository

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrTenantNotFound = errors.New("租户不存在")

// ==================== 租户 Repository ====================

// TenantRepository 租户数据仓库
type TenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository 创建租户仓库
func NewTenantRepository() *TenantRepository {
	return &TenantRepository{db: database.DB}
}

func NewTenantRepositoryWithDB(db *gorm.DB) *TenantRepository {
	return &TenantRepository{db: db}
}

// List 查询租户列表（支持搜索和分页）
func (r *TenantRepository) List(ctx context.Context, keyword string, name, code query.StringFilter, status string, page, pageSize int) ([]model.Tenant, int64, error) {
	var tenants []model.Tenant
	var total int64

	q := r.db.WithContext(ctx).Model(&model.Tenant{})

	if keyword != "" {
		q = q.Where("name ILIKE ? OR code ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	q = query.ApplyStringFilter(q, "name", name)
	q = query.ApplyStringFilter(q, "code", code)
	if status != "" {
		q = q.Where("status = ?", status)
	}

	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	memberCountSubquery := r.db.Table("user_tenant_roles").
		Select("COUNT(DISTINCT user_id)").
		Where("user_tenant_roles.tenant_id = tenants.id")

	// 使用中间结构体接收 member_count（因为 model.Tenant 的 MemberCount 为 gorm:"-"）
	type tenantWithCount struct {
		model.Tenant
		MemberCount int64 `gorm:"column:member_count"`
	}
	var results []tenantWithCount

	err := q.Select("tenants.*, (?) AS member_count", memberCountSubquery).
		Order("created_at ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&results).Error
	if err != nil {
		return nil, 0, err
	}

	tenants = make([]model.Tenant, len(results))
	for i, r := range results {
		tenants[i] = r.Tenant
		tenants[i].MemberCount = r.MemberCount
	}

	return tenants, total, nil
}

// GetByID 根据 ID 获取租户
func (r *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Tenant, error) {
	var tenant model.Tenant
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}

// GetByCode 根据 Code 获取租户
func (r *TenantRepository) GetByCode(ctx context.Context, code string) (*model.Tenant, error) {
	var tenant model.Tenant
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}

// Create 创建租户
func (r *TenantRepository) Create(ctx context.Context, tenant *model.Tenant) error {
	return r.db.WithContext(ctx).Create(tenant).Error
}

// Update 更新租户
func (r *TenantRepository) Update(ctx context.Context, tenant *model.Tenant) error {
	tenant.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(tenant).Error
}

// Delete 删除租户（物理删除）
func (r *TenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Tenant{}).Error
}
