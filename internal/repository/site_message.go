package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SiteMessageRepository 站内信数据仓库
type SiteMessageRepository struct {
	db               *gorm.DB
	platformSettings *PlatformSettingsRepository
}

// NewSiteMessageRepository 创建站内信仓库
func NewSiteMessageRepository() *SiteMessageRepository {
	return &SiteMessageRepository{
		db:               database.DB,
		platformSettings: NewPlatformSettingsRepository(),
	}
}

// List 分页查询站内信（带已读状态），支持 keyword、category、is_read 筛选和排序
// tenantID: 当前用户所属租户，用于过滤目标租户（NULL=广播 或 target_tenant_id=tenantID）
func (r *SiteMessageRepository) List(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, userCreatedAt time.Time, page, pageSize int, keyword, category, isRead, dateFrom, dateTo, sortField, order string) ([]model.SiteMessageWithReadStatus, int64, error) {
	var total int64
	var results []model.SiteMessageWithReadStatus

	ctxTenantID, err := RequireTenantID(ctx)
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
	ctxTenantID, err := RequireTenantID(ctx)
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

// MarkRead 批量标记已读（UPSERT，已读的跳过）
func (r *SiteMessageRepository) MarkRead(ctx context.Context, userID uuid.UUID, messageIDs []uuid.UUID) error {
	if len(messageIDs) == 0 {
		return nil
	}

	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}

	// 先过滤出实际存在的消息 ID，避免外键约束错误
	// 注意：不使用 TenantDB，因为广播消息的 tenant_id 是创建者租户，
	// 在其他租户下标记已读时需要能找到这些消息
	var existingIDs []uuid.UUID
	if err := r.db.WithContext(ctx).
		Model(&model.SiteMessage{}).
		Where("id IN ?", messageIDs).
		Where("target_tenant_id IS NULL OR target_tenant_id = ?", tenantID).
		Pluck("id", &existingIDs).Error; err != nil {
		return err
	}
	if len(existingIDs) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, msgID := range existingIDs {
			read := model.SiteMessageRead{
				TenantID:  &tenantID,
				MessageID: msgID,
				UserID:    userID,
				ReadAt:    time.Now(),
			}
			// ON CONFLICT DO NOTHING — 已读的不重复插入
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "message_id"}, {Name: "user_id"}},
				DoNothing: true,
			}).Create(&read).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// MarkAllRead 全部标记已读（将所有未读消息插入 reads 表）
// tenantID: 当前用户所属租户，用于过滤目标租户
func (r *SiteMessageRepository) MarkAllRead(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, userCreatedAt time.Time) (int64, error) {
	now := time.Now()
	ctxTenantID, err := RequireTenantID(ctx)
	if err != nil {
		return 0, err
	}

	// 构建用户创建时间过滤条件
	userCreatedFilter := ""
	if !userCreatedAt.IsZero() {
		userCreatedFilter = " AND sm.created_at >= ?"
	}

	var result *gorm.DB
	if tenantID != nil {
		baseArgs := []interface{}{ctxTenantID, userID, now, now, *tenantID}
		if !userCreatedAt.IsZero() {
			baseArgs = append(baseArgs, userCreatedAt)
		}
		result = r.db.WithContext(ctx).Exec(`
			INSERT INTO site_message_reads (id, tenant_id, message_id, user_id, read_at)
			SELECT gen_random_uuid(), ?, sm.id, ?, ? FROM site_messages sm
			WHERE (sm.expires_at IS NULL OR sm.expires_at > ?)
			AND (sm.target_tenant_id IS NULL OR sm.target_tenant_id = ?)
			AND NOT EXISTS (SELECT 1 FROM site_message_reads WHERE tenant_id = $1 AND message_id = sm.id AND user_id = $2)`+userCreatedFilter+`
		`, baseArgs...)
	} else {
		baseArgs := []interface{}{ctxTenantID, userID, now, now}
		if !userCreatedAt.IsZero() {
			baseArgs = append(baseArgs, userCreatedAt)
		}
		result = r.db.WithContext(ctx).Exec(`
			INSERT INTO site_message_reads (id, tenant_id, message_id, user_id, read_at)
			SELECT gen_random_uuid(), ?, sm.id, ?, ? FROM site_messages sm
			WHERE (sm.expires_at IS NULL OR sm.expires_at > ?)
			AND NOT EXISTS (SELECT 1 FROM site_message_reads WHERE tenant_id = $1 AND message_id = sm.id AND user_id = $2)`+userCreatedFilter+`
		`, baseArgs...)
	}

	return result.RowsAffected, result.Error
}

// Create 创建站内信
func (r *SiteMessageRepository) Create(ctx context.Context, msg *model.SiteMessage) error {
		// 自动设置 tenant_id；平台广播消息允许为空租户。
		if msg.TenantID == nil {
			tenantID, ok := TenantIDFromContextOK(ctx)
			if !ok {
				goto retention
			}
			msg.TenantID = &tenantID
		}
	retention:
		// 如果没有设置过期时间，从 platform_settings 获取保留天数
		if msg.ExpiresAt == nil {
		retentionDays := r.platformSettings.GetIntValue(ctx, "site_message.retention_days", 90)
		if retentionDays > 0 {
			expiresAt := time.Now().AddDate(0, 0, retentionDays)
			msg.ExpiresAt = &expiresAt
		}
	}
	return r.db.WithContext(ctx).Create(msg).Error
}

// CreateBatch 在一个事务中批量创建多条站内信
func (r *SiteMessageRepository) CreateBatch(ctx context.Context, msgs []*model.SiteMessage) error {
	if len(msgs) == 0 {
		return nil
	}

	tenantID, hasTenant := TenantIDFromContextOK(ctx)
	retentionDays := r.platformSettings.GetIntValue(ctx, "site_message.retention_days", 90)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, msg := range msgs {
			// 自动设置 tenant_id
			if msg.TenantID == nil && hasTenant {
				tid := tenantID
				msg.TenantID = &tid
			}
			// 自动设置过期时间
			if msg.ExpiresAt == nil && retentionDays > 0 {
				expiresAt := time.Now().AddDate(0, 0, retentionDays)
				msg.ExpiresAt = &expiresAt
			}
			if err := tx.Create(msg).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// CleanExpired 清理已过期的站内信（级联删除 reads）
func (r *SiteMessageRepository) CleanExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).
		Delete(&model.SiteMessage{})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		logger.Info("站内信过期清理：已删除 %d 条过期消息", result.RowsAffected)
	}
	return result.RowsAffected, nil
}
