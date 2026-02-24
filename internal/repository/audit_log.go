package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 风险等级常量
const (
	RiskLevelLow      = "low"      // 低风险 — 查看、登录、导出等只读操作
	RiskLevelMedium   = "medium"   // 中风险 — 创建、更新、同步等常规写操作
	RiskLevelHigh     = "high"     // 高风险 — 删除、禁用、重置密码等危险操作
	RiskLevelCritical = "critical" // 极高风险 — 提权、批量操作、角色变更等最高危操作
)

// RiskRule 操作风险规则（action + resource_type → level + reason）
type RiskRule struct {
	Action       string // 操作类型，"*" 表示任意
	ResourceType string // 资源类型，"*" 表示任意
	Level        string // 风险等级：low / medium / high / critical
	Reason       string // 风险原因
}

// RiskRules 操作风险规则列表（按优先级排列：精确匹配 > 通配符）
var RiskRules = []RiskRule{
	// ══════════ 极高风险 (critical) ══════════
	// 提权操作
	{"impersonation_enter", "impersonation", RiskLevelCritical, "提权进入租户"},
	{"impersonation_exit", "impersonation", RiskLevelCritical, "提权退出租户"},
	// 角色权限变更
	{"assign_role", "users", RiskLevelCritical, "变更用户角色"},
	{"assign_permission", "roles", RiskLevelCritical, "变更角色权限"},

	// ══════════ 高风险 (high) ══════════
	// 删除操作 — 任意资源删除
	{"delete", "*", RiskLevelHigh, "删除操作"},
	// 用户管理
	{"reset_password", "users", RiskLevelHigh, "管理员重置用户密码"},
	{"disable", "*", RiskLevelHigh, "禁用资源"},
	{"deactivate", "plugins", RiskLevelHigh, "停用插件"},
	// 取消执行
	{"cancel", "*", RiskLevelHigh, "取消执行中的任务"},

	// ══════════ 中风险 (medium) ══════════
	// 执行操作
	{"execute", "execution-tasks", RiskLevelMedium, "执行指令/Playbook"},
	// 自愈相关
	{"trigger", "incidents", RiskLevelMedium, "手动触发自愈流程"},
	{"dismiss", "incidents", RiskLevelMedium, "忽略待触发工单"},
	{"approve", "healing", RiskLevelMedium, "审批通过自愈流程"},
	{"reject", "healing", RiskLevelMedium, "审批拒绝自愈流程"},
	{"dry_run", "healing", RiskLevelMedium, "自愈流程试运行"},
	// 创建、更新、同步操作（通配符 — 最低优先级）
	{"create", "*", RiskLevelMedium, "创建资源"},
	{"update", "*", RiskLevelMedium, "更新资源"},
	{"sync", "*", RiskLevelMedium, "同步操作"},
}

// 保留旧名以兼容
type HighRiskRule = RiskRule

var HighRiskRules = RiskRules

// AuditLogListOptions 审计日志查询选项
type AuditLogListOptions struct {
	Page                 int
	PageSize             int
	Search               query.StringFilter // 模糊搜索（username/resource_name/request_path）
	Category             string             // 过滤分类 (login/operation)
	Action               string             // 过滤操作类型
	ResourceType         string             // 过滤资源类型
	ExcludeActions       []string           // 排除操作类型
	ExcludeResourceTypes []string           // 排除资源类型
	Username             query.StringFilter // 过滤用户名
	UserID               *uuid.UUID         // 过滤用户 ID
	Status               string             // 过滤状态 (success/failed)
	RiskLevel            string             // 过滤风险等级 (high/normal)
	RequestPath          query.StringFilter // 过滤请求路径
	CreatedAfter         *time.Time         // 开始时间
	CreatedBefore        *time.Time         // 结束时间
	SortBy               string             // 排序字段
	SortOrder            string             // 排序方向 (asc/desc)
}

// AuditLogRepository 审计日志仓库
type AuditLogRepository struct {
	db *gorm.DB
}

// NewAuditLogRepository 创建审计日志仓库
func NewAuditLogRepository() *AuditLogRepository {
	return &AuditLogRepository{db: database.DB}
}

// Create 创建审计日志
func (r *AuditLogRepository) Create(ctx context.Context, log *model.AuditLog) error {
	if log.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		log.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByID 根据 ID 获取审计日志
func (r *AuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AuditLog, error) {
	var log model.AuditLog
	err := TenantDB(r.db, ctx).Preload("User").First(&log, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

// List 分页查询审计日志
func (r *AuditLogRepository) List(ctx context.Context, opts *AuditLogListOptions) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64

	q := TenantDB(r.db, ctx).Model(&model.AuditLog{})

	// 过滤条件
	if opts.Category != "" {
		q = q.Where("category = ?", opts.Category)
	}
	if opts.Action != "" {
		q = q.Where("action = ?", opts.Action)
	}
	if opts.ResourceType != "" {
		q = q.Where("resource_type = ?", opts.ResourceType)
	}
	if len(opts.ExcludeActions) > 0 {
		q = q.Where("action NOT IN ?", opts.ExcludeActions)
	}
	if len(opts.ExcludeResourceTypes) > 0 {
		q = q.Where("resource_type NOT IN ?", opts.ExcludeResourceTypes)
	}
	if !opts.Username.IsEmpty() {
		q = query.ApplyStringFilter(q, "username", opts.Username)
	}
	if opts.UserID != nil {
		q = q.Where("user_id = ?", *opts.UserID)
	}
	if opts.Status != "" {
		q = q.Where("status = ?", opts.Status)
	}
	if opts.CreatedAfter != nil {
		q = q.Where("created_at >= ?", *opts.CreatedAfter)
	}
	if opts.CreatedBefore != nil {
		q = q.Where("created_at <= ?", *opts.CreatedBefore)
	}
	if !opts.RequestPath.IsEmpty() {
		q = query.ApplyStringFilter(q, "request_path", opts.RequestPath)
	}

	// 高危过滤
	switch opts.RiskLevel {
	case "high":
		q = q.Where(buildHighRiskCondition())
	case "normal":
		q = q.Where(fmt.Sprintf("NOT (%s)", buildHighRiskCondition()))
	}

	// 搜索
	if !opts.Search.IsEmpty() {
		q = query.ApplyMultiStringFilter(q, []string{"username", "resource_name", "request_path"}, opts.Search)
	}

	// 计数
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	sortBy := "created_at"
	sortOrder := "DESC"
	allowedSortFields := map[string]bool{
		"created_at":    true,
		"action":        true,
		"resource_type": true,
		"username":      true,
		"status":        true,
	}
	if opts.SortBy != "" && allowedSortFields[opts.SortBy] {
		sortBy = opts.SortBy
	}
	if opts.SortOrder == "asc" || opts.SortOrder == "ASC" {
		sortOrder = "ASC"
	}
	q = q.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// 分页
	offset := (opts.Page - 1) * opts.PageSize
	if err := q.Offset(offset).Limit(opts.PageSize).Preload("User").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// ==================== 聚合查询 ====================

// AuditStats 审计统计概览
type AuditStats struct {
	TotalCount    int64        `json:"total_count"`
	SuccessCount  int64        `json:"success_count"`
	FailedCount   int64        `json:"failed_count"`
	HighRiskCount int64        `json:"high_risk_count"`
	ActionStats   []ActionStat `json:"action_stats"`
	TodayCount    int64        `json:"today_count"`
	WeekCount     int64        `json:"week_count"`
}

// ActionStat 按操作分组统计
type ActionStat struct {
	Action string `json:"action" gorm:"column:action"`
	Count  int64  `json:"count" gorm:"column:count"`
}

// GetStats 获取审计统计概览
func (r *AuditLogRepository) GetStats(ctx context.Context) (*AuditStats, error) {
	stats := &AuditStats{}

	// 总数
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	newDB().Model(&model.AuditLog{}).Count(&stats.TotalCount)

	// 成功/失败
	newDB().Model(&model.AuditLog{}).Where("status = ?", "success").Count(&stats.SuccessCount)
	newDB().Model(&model.AuditLog{}).Where("status = ?", "failed").Count(&stats.FailedCount)

	// 高危操作数
	newDB().Model(&model.AuditLog{}).
		Where(buildHighRiskCondition()).
		Count(&stats.HighRiskCount)

	// 按操作类型分组
	newDB().Model(&model.AuditLog{}).
		Select("action, count(*) as count").
		Group("action").
		Order("count DESC").
		Scan(&stats.ActionStats)

	// 今日和本周
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))
	newDB().Model(&model.AuditLog{}).Where("created_at >= ?", todayStart).Count(&stats.TodayCount)
	newDB().Model(&model.AuditLog{}).Where("created_at >= ?", weekStart).Count(&stats.WeekCount)

	return stats, nil
}

// UserRanking 用户操作排行
type UserRanking struct {
	UserID   string `json:"user_id" gorm:"column:user_id"`
	Username string `json:"username" gorm:"column:username"`
	Count    int64  `json:"count" gorm:"column:count"`
}

// GetUserRanking 获取用户操作排行榜
func (r *AuditLogRepository) GetUserRanking(ctx context.Context, limit int, days int) ([]UserRanking, error) {
	var rankings []UserRanking

	query := TenantDB(r.db, ctx).Model(&model.AuditLog{}).
		Select("user_id, username, count(*) as count")

	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		query = query.Where("created_at >= ?", since)
	}

	err := query.
		Where("user_id IS NOT NULL").
		Group("user_id, username").
		Order("count DESC").
		Limit(limit).
		Scan(&rankings).Error

	return rankings, err
}

// ActionGroupItem 操作分组明细
type ActionGroupItem struct {
	Action       string `json:"action" gorm:"column:action"`
	ResourceType string `json:"resource_type" gorm:"column:resource_type"`
	Username     string `json:"username" gorm:"column:username"`
	Count        int64  `json:"count" gorm:"column:count"`
}

// GetActionGrouping 按操作类型 + 用户分组统计
func (r *AuditLogRepository) GetActionGrouping(ctx context.Context, action string, days int) ([]ActionGroupItem, error) {
	var items []ActionGroupItem

	query := TenantDB(r.db, ctx).Model(&model.AuditLog{}).
		Select("action, resource_type, username, count(*) as count")

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		query = query.Where("created_at >= ?", since)
	}

	err := query.
		Group("action, resource_type, username").
		Order("count DESC").
		Scan(&items).Error

	return items, err
}

// ResourceTypeGroupItem 资源类型分组
type ResourceTypeGroupItem struct {
	ResourceType string `json:"resource_type" gorm:"column:resource_type"`
	Count        int64  `json:"count" gorm:"column:count"`
}

// GetResourceTypeStats 按资源类型统计
func (r *AuditLogRepository) GetResourceTypeStats(ctx context.Context, days int) ([]ResourceTypeGroupItem, error) {
	var items []ResourceTypeGroupItem

	query := TenantDB(r.db, ctx).Model(&model.AuditLog{}).
		Select("resource_type, count(*) as count")

	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		query = query.Where("created_at >= ?", since)
	}

	err := query.
		Group("resource_type").
		Order("count DESC").
		Scan(&items).Error

	return items, err
}

// TrendItem 趋势数据
type TrendItem struct {
	Date  string `json:"date" gorm:"column:date"`
	Count int64  `json:"count" gorm:"column:count"`
}

// GetTrend 获取操作趋势（按天分组）
func (r *AuditLogRepository) GetTrend(ctx context.Context, days int) ([]TrendItem, error) {
	var items []TrendItem

	since := time.Now().AddDate(0, 0, -days)

	err := TenantDB(r.db, ctx).Model(&model.AuditLog{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, count(*) as count").
		Where("created_at >= ?", since).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&items).Error

	return items, err
}

// HighRiskLog 高危操作记录（带风险原因）
type HighRiskLog struct {
	model.AuditLog
	RiskReason string `json:"risk_reason" gorm:"-"`
}

// GetHighRiskLogs 获取高危操作日志
func (r *AuditLogRepository) GetHighRiskLogs(ctx context.Context, page, pageSize int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.AuditLog{}).
		Where(buildHighRiskCondition())

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Preload("User").
		Find(&logs).Error

	return logs, total, err
}

// IsHighRisk 判断一条审计日志是否属于高危（high 或 critical）
// 保留此函数以兼容已有调用
func IsHighRisk(action, resourceType string) bool {
	level := GetRiskLevel(action, resourceType)
	return level == RiskLevelHigh || level == RiskLevelCritical
}

// GetRiskLevel 获取操作的风险等级（四级）
// 返回 low / medium / high / critical
func GetRiskLevel(action, resourceType string) string {
	for _, rule := range RiskRules {
		if (rule.Action == "*" || rule.Action == action) &&
			(rule.ResourceType == "*" || rule.ResourceType == resourceType) {
			return rule.Level
		}
	}
	return RiskLevelLow // 默认低风险
}

// GetRiskReason 获取风险原因描述
func GetRiskReason(action, resourceType string) string {
	for _, rule := range RiskRules {
		if (rule.Action == "*" || rule.Action == action) &&
			(rule.ResourceType == "*" || rule.ResourceType == resourceType) {
			return rule.Reason
		}
	}
	return ""
}

// buildHighRiskCondition 构建高危操作的 SQL WHERE 条件（high + critical）
func buildHighRiskCondition() string {
	conditions := make([]string, 0, len(RiskRules))
	for _, rule := range RiskRules {
		if rule.Level != RiskLevelHigh && rule.Level != RiskLevelCritical {
			continue
		}
		if rule.Action == "*" && rule.ResourceType == "*" {
			return "1=1"
		} else if rule.Action == "*" {
			conditions = append(conditions, fmt.Sprintf("resource_type = '%s'", rule.ResourceType))
		} else if rule.ResourceType == "*" {
			conditions = append(conditions, fmt.Sprintf("action = '%s'", rule.Action))
		} else {
			conditions = append(conditions, fmt.Sprintf("(action = '%s' AND resource_type = '%s')", rule.Action, rule.ResourceType))
		}
	}
	if len(conditions) == 0 {
		return "1=0" // 无规则匹配
	}
	return strings.Join(conditions, " OR ")
}

// ==================== 用户个人中心查询 ====================

// GetUserLoginHistory 获取指定用户的登录历史
// tenantID 非零时按租户过滤（租户用户场景）
func (r *AuditLogRepository) GetUserLoginHistory(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, limit int) ([]model.AuditLog, error) {
	if limit <= 0 {
		limit = 10
	}
	var logs []model.AuditLog
	q := r.db.WithContext(ctx).
		Where("user_id = ? AND category = ?", userID, "login")
	if tenantID != uuid.Nil {
		q = q.Where("tenant_id = ?", tenantID)
	}
	err := q.Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// GetUserActivities 获取指定用户的操作记录（排除 login/logout）
// tenantID 非零时按租户过滤（租户用户场景）
func (r *AuditLogRepository) GetUserActivities(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, limit int) ([]model.AuditLog, error) {
	if limit <= 0 {
		limit = 15
	}
	var logs []model.AuditLog
	q := r.db.WithContext(ctx).
		Where("user_id = ? AND category = ?", userID, "operation")
	if tenantID != uuid.Nil {
		q = q.Where("tenant_id = ?", tenantID)
	}
	err := q.Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
