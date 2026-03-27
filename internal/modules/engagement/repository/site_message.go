package repository

import (
	"context"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SiteMessageRepository 站内信数据仓库
type SiteMessageRepository struct {
	db               *gorm.DB
	platformSettings *settingsrepo.PlatformSettingsRepository
}

type SiteMessageRepositoryDeps struct {
	DB               *gorm.DB
	PlatformSettings *settingsrepo.PlatformSettingsRepository
}

func NewSiteMessageRepositoryWithDB(db *gorm.DB) *SiteMessageRepository {
	return NewSiteMessageRepositoryWithDeps(SiteMessageRepositoryDeps{
		DB:               db,
		PlatformSettings: settingsrepo.NewPlatformSettingsRepositoryWithDB(db),
	})
}

func NewSiteMessageRepositoryWithDeps(deps SiteMessageRepositoryDeps) *SiteMessageRepository {
	switch {
	case deps.DB == nil:
		panic("site message repository requires db")
	case deps.PlatformSettings == nil:
		panic("site message repository requires platform settings repository")
	}
	return &SiteMessageRepository{
		db:               deps.DB,
		platformSettings: deps.PlatformSettings,
	}
}

// List 分页查询站内信（带已读状态），支持 keyword、category、is_read 筛选和排序
// tenantID: 当前用户所属租户，用于过滤目标租户（NULL=广播 或 target_tenant_id=tenantID）
func (r *SiteMessageRepository) List(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, userCreatedAt time.Time, page, pageSize int, keyword, category, isRead, dateFrom, dateTo, sortField, order string) ([]model.SiteMessageWithReadStatus, int64, error) {
	var total int64
	var results []model.SiteMessageWithReadStatus

	ctxTenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, 0, err
	}
	baseQuery := r.siteMessageListBaseQuery(ctx, tenantID, userCreatedAt, keyword, category, dateFrom, dateTo)
	baseQuery = applySiteMessageReadFilter(baseQuery, isRead, ctxTenantID, userID)
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = baseQuery.
		Select(`sm.id, sm.category, sm.title, sm.content, sm.created_at, sm.expires_at,
			CASE WHEN smr.id IS NOT NULL THEN true ELSE false END AS is_read`).
		Joins("LEFT JOIN site_message_reads AS smr ON smr.tenant_id = ? AND smr.message_id = sm.id AND smr.user_id = ?", ctxTenantID, userID).
		Order(siteMessageOrderClause(sortField, order)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&results).Error

	return results, total, err
}

func (r *SiteMessageRepository) siteMessageListBaseQuery(ctx context.Context, tenantID *uuid.UUID, userCreatedAt time.Time, keyword, category, dateFrom, dateTo string) *gorm.DB {
	baseQuery := r.db.WithContext(ctx).Table("site_messages AS sm").
		Where("(sm.expires_at IS NULL OR sm.expires_at > ?)", time.Now())
	if !userCreatedAt.IsZero() {
		baseQuery = baseQuery.Where("sm.created_at >= ?", userCreatedAt)
	}
	if tenantID != nil {
		baseQuery = baseQuery.Where("(sm.target_tenant_id IS NULL OR sm.target_tenant_id = ?)", *tenantID)
	}
	if keyword != "" {
		baseQuery = baseQuery.Where("sm.title ILIKE ?", "%"+keyword+"%")
	}
	if category != "" {
		baseQuery = baseQuery.Where("sm.category = ?", category)
	}
	return applySiteMessageDateFilter(baseQuery, dateFrom, dateTo)
}

func applySiteMessageDateFilter(q *gorm.DB, dateFrom, dateTo string) *gorm.DB {
	if dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			q = q.Where("sm.created_at >= ?", t)
		}
	}
	if dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			q = q.Where("sm.created_at < ?", t.AddDate(0, 0, 1))
		}
	}
	return q
}

func applySiteMessageReadFilter(q *gorm.DB, isRead string, tenantID, userID uuid.UUID) *gorm.DB {
	switch isRead {
	case "true":
		return q.Where("EXISTS (SELECT 1 FROM site_message_reads WHERE tenant_id = ? AND message_id = sm.id AND user_id = ?)", tenantID, userID)
	case "false":
		return q.Where("NOT EXISTS (SELECT 1 FROM site_message_reads WHERE tenant_id = ? AND message_id = sm.id AND user_id = ?)", tenantID, userID)
	default:
		return q
	}
}

func siteMessageOrderClause(sortField, order string) string {
	if sortField == "created_at" {
		if order == "asc" {
			return "sm.created_at ASC"
		}
	}
	return "sm.created_at DESC"
}

// GetUnreadCount 获取当前用户的未读站内信数量（轻量查询）
// tenantID: 当前用户所属租户，用于过滤目标租户
func (r *SiteMessageRepository) GetUnreadCount(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, userCreatedAt time.Time) (int64, error) {
	var count int64
	ctxTenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return 0, err
	}
	query := r.db.WithContext(ctx).Table("site_messages AS sm").
		Where("(sm.expires_at IS NULL OR sm.expires_at > ?)", time.Now()).
		Where("NOT EXISTS (SELECT 1 FROM site_message_reads WHERE tenant_id = ? AND message_id = sm.id AND user_id = ?)", ctxTenantID, userID)

	// 只计算用户创建时间之后的消息
	if !userCreatedAt.IsZero() {
		query = query.Where("sm.created_at >= ?", userCreatedAt)
	}

	if tenantID != nil {
		query = query.Where("(sm.target_tenant_id IS NULL OR sm.target_tenant_id = ?)", *tenantID)
	}

	err = query.Count(&count).Error
	return count, err
}
