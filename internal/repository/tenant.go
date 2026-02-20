package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== 租户 Repository ====================

// TenantRepository 租户数据仓库
type TenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository 创建租户仓库
func NewTenantRepository() *TenantRepository {
	return &TenantRepository{db: database.DB}
}

// List 查询租户列表（支持搜索和分页）
func (r *TenantRepository) List(ctx context.Context, keyword string, page, pageSize int) ([]model.Tenant, int64, error) {
	var tenants []model.Tenant
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Tenant{})

	if keyword != "" {
		query = query.Where("name ILIKE ? OR code ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
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

	err := query.Select("tenants.*, (?) AS member_count", memberCountSubquery).
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
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}

// GetByCode 根据 Code 获取租户
func (r *TenantRepository) GetByCode(ctx context.Context, code string) (*model.Tenant, error) {
	var tenant model.Tenant
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&tenant).Error
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

// ==================== 租户成员 ====================

// ListMembers 查询租户成员（带角色和用户信息）
// 注：platform_admin 是全局角色，不在 user_tenant_roles 中，无需过滤
func (r *TenantRepository) ListMembers(ctx context.Context, tenantID uuid.UUID) ([]model.UserTenantRole, error) {
	var members []model.UserTenantRole
	err := r.db.WithContext(ctx).
		Preload("Role").
		Preload("Tenant").
		Where("user_tenant_roles.tenant_id = ?", tenantID).
		Find(&members).Error
	if err != nil {
		return nil, err
	}

	// 手动批量查 users 表（绕过 Preload("User") 可能因 context 导致的问题）
	if len(members) == 0 {
		return members, nil
	}
	userIDs := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		userIDs = append(userIDs, m.UserID)
	}
	var users []model.User
	if err2 := database.DB.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err2 == nil {
		userMap := make(map[uuid.UUID]model.User, len(users))
		for _, u := range users {
			userMap[u.ID] = u
		}
		for i := range members {
			if u, ok := userMap[members[i].UserID]; ok {
				members[i].User = u
			}
		}
	}

	return members, nil
}

// AddMember 添加成员到租户
func (r *TenantRepository) AddMember(ctx context.Context, userID, tenantID, roleID uuid.UUID) error {
	utr := model.UserTenantRole{
		UserID:   userID,
		TenantID: tenantID,
		RoleID:   roleID,
	}
	return r.db.WithContext(ctx).Create(&utr).Error
}

// RemoveMember 从租户移除成员（删除该用户在此租户的所有角色）
func (r *TenantRepository) RemoveMember(ctx context.Context, userID, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Delete(&model.UserTenantRole{}).Error
}

// GetMember 查询用户在租户内的角色记录（判断是否已是成员）
func (r *TenantRepository) GetMember(ctx context.Context, userID, tenantID uuid.UUID) (*model.UserTenantRole, error) {
	var utr model.UserTenantRole
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&utr).Error
	if err != nil {
		return nil, err
	}
	return &utr, nil
}

// UpdateMemberRole 更新用户在租户内的角色（升级/降级）
func (r *TenantRepository) UpdateMemberRole(ctx context.Context, userID, tenantID, roleID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.UserTenantRole{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Update("role_id", roleID).Error
}

// GetUserTenants 获取用户所属的租户列表
// search 可选，不为空时对 tenants.name 和 tenants.code 做 ILIKE 模糊匹配
func (r *TenantRepository) GetUserTenants(ctx context.Context, userID uuid.UUID, search string) ([]model.Tenant, error) {
	var tenants []model.Tenant
	query := r.db.WithContext(ctx).
		Table("tenants").
		Joins("INNER JOIN user_tenant_roles ON user_tenant_roles.tenant_id = tenants.id").
		Where("user_tenant_roles.user_id = ?", userID).
		Where("tenants.status = ?", model.TenantStatusActive)

	if search != "" {
		query = query.Where("tenants.name ILIKE ? OR tenants.code ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	err := query.Group("tenants.id").Find(&tenants).Error
	return tenants, err
}
