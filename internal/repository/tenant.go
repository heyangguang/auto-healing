package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
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

// ListSimpleMembers 获取租户下简要用户列表（轻量接口，用于下拉选择）
func (r *TenantRepository) ListSimpleMembers(ctx context.Context, tenantID uuid.UUID, search string, status string) ([]SimpleUser, error) {
	var users []SimpleUser

	query := r.db.WithContext(ctx).
		Table("users").
		Select(`users.id, users.username, users.display_name, users.status`).
		Joins("INNER JOIN user_tenant_roles ON user_tenant_roles.user_id = users.id").
		Where("user_tenant_roles.tenant_id = ?", tenantID)

	if status != "" {
		query = query.Where("users.status = ?", status)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("users.username ILIKE ? OR users.display_name ILIKE ?", like, like)
	}

	err := query.Order("users.username ASC").Limit(500).Scan(&users).Error
	return users, err
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

// GetUserAllRoles 获取用户在所有租户中的角色（去重）
func (r *TenantRepository) GetUserAllRoles(ctx context.Context, userID uuid.UUID) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).
		Distinct("roles.*").
		Table("roles").
		Joins("INNER JOIN user_tenant_roles ON user_tenant_roles.role_id = roles.id").
		Where("user_tenant_roles.user_id = ?", userID).
		Find(&roles).Error
	return roles, err
}

// GetUserTenantRoles 获取用户在指定租户中的角色
func (r *TenantRepository) GetUserTenantRoles(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).
		Distinct("roles.*").
		Table("roles").
		Joins("INNER JOIN user_tenant_roles ON user_tenant_roles.role_id = roles.id").
		Where("user_tenant_roles.user_id = ? AND user_tenant_roles.tenant_id = ?", userID, tenantID).
		Find(&roles).Error
	return roles, err
}

// ==================== 租户运营统计 ====================

// CountTenantMembers 统计某租户的成员数
func (r *TenantRepository) CountTenantMembers(ctx context.Context, tenantID uuid.UUID) int64 {
	var count int64
	r.db.WithContext(ctx).
		Model(&model.UserTenantRole{}).
		Where("tenant_id = ?", tenantID).
		Distinct("user_id").
		Count(&count)
	return count
}

// CountTenantTable 统计某租户在指定表中的记录数（通用方法）
// 支持的表：healing_rules, healing_instances, task_templates, audit_logs 等
func (r *TenantRepository) CountTenantTable(ctx context.Context, tenantID uuid.UUID, tableName string) int64 {
	// 先检查表是否存在，避免对不存在的表查询产生 ERROR 日志
	var exists bool
	r.db.WithContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = ?)", tableName,
	).Scan(&exists)
	if !exists {
		return 0
	}

	var count int64
	r.db.WithContext(ctx).
		Table(tableName).
		Where("tenant_id = ?", tenantID).
		Count(&count)
	return count
}

// GetTenantLastActivity 获取租户最近一条审计日志的时间
func (r *TenantRepository) GetTenantLastActivity(ctx context.Context, tenantID uuid.UUID) *string {
	var result sql.NullString
	err := r.db.WithContext(ctx).
		Table("audit_logs").
		Select("to_char(MAX(created_at), 'YYYY-MM-DD HH24:MI:SS')").
		Where("tenant_id = ?", tenantID).
		Scan(&result).Error
	if err != nil || !result.Valid {
		return nil
	}
	return &result.String
}

// CountTenantTableWhere 统计某租户在指定表中满足额外条件的记录数
// extraWhere 为原始 SQL 片段（仅内部使用，不接受用户输入）
func (r *TenantRepository) CountTenantTableWhere(ctx context.Context, tenantID uuid.UUID, tableName string, extraWhere string) int64 {
	var exists bool
	r.db.WithContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = ?)", tableName,
	).Scan(&exists)
	if !exists {
		return 0
	}

	var count int64
	r.db.WithContext(ctx).
		Table(tableName).
		Where("tenant_id = ?", tenantID).
		Where(extraWhere).
		Count(&count)
	return count
}

// ==================== 平台趋势统计 ====================

// trendRow 趋势查询中间结构
type trendRow struct {
	Date  string `gorm:"column:date"`
	Count int64  `gorm:"column:cnt"`
}

// GetTrendByDay 按天统计某张表最近 N 天的记录数（跨所有租户）
func (r *TenantRepository) GetTrendByDay(ctx context.Context, tableName string, days int) ([]string, []int64, error) {
	var exists bool
	r.db.WithContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = ?)", tableName,
	).Scan(&exists)
	if !exists {
		return fillEmptyTrend(days), fillZeroCounts(days), nil
	}

	var rows []trendRow
	sql := fmt.Sprintf(
		`SELECT TO_CHAR(DATE(created_at), 'MM/DD') AS date, COUNT(*) AS cnt
		 FROM %s
		 WHERE created_at >= NOW() - INTERVAL '%d days'
		 GROUP BY DATE(created_at)
		 ORDER BY DATE(created_at) ASC`, tableName, days)

	if err := r.db.WithContext(ctx).Raw(sql).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}

	dates, counts := mergeTrendRows(rows, days)
	return dates, counts, nil
}

// GetTrendByDayWhere 带条件按天统计（用于安全审计子集）
func (r *TenantRepository) GetTrendByDayWhere(ctx context.Context, tableName string, days int, extraWhere string) ([]string, []int64, error) {
	var exists bool
	r.db.WithContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = ?)", tableName,
	).Scan(&exists)
	if !exists {
		return fillEmptyTrend(days), fillZeroCounts(days), nil
	}

	var rows []trendRow
	sql := fmt.Sprintf(
		`SELECT TO_CHAR(DATE(created_at), 'MM/DD') AS date, COUNT(*) AS cnt
		 FROM %s
		 WHERE created_at >= NOW() - INTERVAL '%d days' AND (%s)
		 GROUP BY DATE(created_at)
		 ORDER BY DATE(created_at) ASC`, tableName, days, extraWhere)

	if err := r.db.WithContext(ctx).Raw(sql).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}

	dates, counts := mergeTrendRows(rows, days)
	return dates, counts, nil
}

// mergeTrendRows 将数据库返回的稀疏日期数据补全为连续 N 天
func mergeTrendRows(rows []trendRow, days int) ([]string, []int64) {
	dateMap := make(map[string]int64, len(rows))
	for _, r := range rows {
		dateMap[r.Date] = r.Count
	}

	dates := make([]string, days)
	counts := make([]int64, days)
	now := time.Now()
	for i := 0; i < days; i++ {
		d := now.AddDate(0, 0, -(days - 1 - i))
		label := fmt.Sprintf("%02d/%02d", d.Month(), d.Day())
		dates[i] = label
		counts[i] = dateMap[label]
	}
	return dates, counts
}

// fillEmptyTrend 生成空的 N 天日期标签
func fillEmptyTrend(days int) []string {
	dates := make([]string, days)
	now := time.Now()
	for i := 0; i < days; i++ {
		d := now.AddDate(0, 0, -(days - 1 - i))
		dates[i] = fmt.Sprintf("%02d/%02d", d.Month(), d.Day())
	}
	return dates
}

// fillZeroCounts 生成 N 个 0 的计数
func fillZeroCounts(days int) []int64 {
	return make([]int64, days)
}
