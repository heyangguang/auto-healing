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
func (r *SiteMessageRepository) List(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, userCreatedAt time.Time, page, pageSize int, keyword, category, isRead, sortField, order string) ([]model.SiteMessageWithReadStatus, int64, error) {
	var total int64
	var results []model.SiteMessageWithReadStatus

	baseQuery := r.db.WithContext(ctx).Table("site_messages AS sm").
		Where("(sm.expires_at IS NULL OR sm.expires_at > ?)", time.Now())

	// 只显示用户创建时间之后的消息（新用户不看旧公告）
	if !userCreatedAt.IsZero() {
		baseQuery = baseQuery.Where("sm.created_at >= ?", userCreatedAt)
	}

	// 租户过滤：只看全局广播（target_tenant_id IS NULL）或定向到自己租户的消息
	if tenantID != nil {
		baseQuery = baseQuery.Where("(sm.target_tenant_id IS NULL OR sm.target_tenant_id = ?)", *tenantID)
	}

	// 筛选条件
	if keyword != "" {
		baseQuery = baseQuery.Where("sm.title ILIKE ?", "%"+keyword+"%")
	}
	if category != "" {
		baseQuery = baseQuery.Where("sm.category = ?", category)
	}

	// 已读状态过滤
	switch isRead {
	case "true":
		baseQuery = baseQuery.Where("EXISTS (SELECT 1 FROM site_message_reads WHERE message_id = sm.id AND user_id = ?)", userID)
	case "false":
		baseQuery = baseQuery.Where("NOT EXISTS (SELECT 1 FROM site_message_reads WHERE message_id = sm.id AND user_id = ?)", userID)
	}

	// 先计算总数
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询带已读状态的列表
	// 排序：支持 sort + order 参数，白名单验证
	orderClause := "sm.created_at DESC" // 默认排序
	allowedSortFields := map[string]bool{"created_at": true}
	if sortField != "" && allowedSortFields[sortField] {
		orderDir := "DESC"
		if order == "asc" {
			orderDir = "ASC"
		}
		orderClause = "sm." + sortField + " " + orderDir
	}

	err := baseQuery.
		Select(`sm.id, sm.category, sm.title, sm.content, sm.created_at, sm.expires_at,
			CASE WHEN smr.id IS NOT NULL THEN true ELSE false END AS is_read`).
		Joins("LEFT JOIN site_message_reads AS smr ON smr.message_id = sm.id AND smr.user_id = ?", userID).
		Order(orderClause).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&results).Error

	return results, total, err
}

// GetUnreadCount 获取当前用户的未读站内信数量（轻量查询）
// tenantID: 当前用户所属租户，用于过滤目标租户
func (r *SiteMessageRepository) GetUnreadCount(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, userCreatedAt time.Time) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Table("site_messages AS sm").
		Where("(sm.expires_at IS NULL OR sm.expires_at > ?)", time.Now()).
		Where("NOT EXISTS (SELECT 1 FROM site_message_reads WHERE message_id = sm.id AND user_id = ?)", userID)

	// 只计算用户创建时间之后的消息
	if !userCreatedAt.IsZero() {
		query = query.Where("sm.created_at >= ?", userCreatedAt)
	}

	if tenantID != nil {
		query = query.Where("(sm.target_tenant_id IS NULL OR sm.target_tenant_id = ?)", *tenantID)
	}

	err := query.Count(&count).Error
	return count, err
}

// MarkRead 批量标记已读（UPSERT，已读的跳过）
func (r *SiteMessageRepository) MarkRead(ctx context.Context, userID uuid.UUID, messageIDs []uuid.UUID) error {
	if len(messageIDs) == 0 {
		return nil
	}

	tenantID := TenantIDFromContext(ctx)

	// 先过滤出实际存在的消息 ID，避免外键约束错误
	// 注意：不使用 TenantDB，因为广播消息的 tenant_id 是创建者租户，
	// 在其他租户下标记已读时需要能找到这些消息
	var existingIDs []uuid.UUID
	if err := r.db.WithContext(ctx).
		Model(&model.SiteMessage{}).
		Where("id IN ?", messageIDs).
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
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&read).Error; err != nil {
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
	ctxTenantID := TenantIDFromContext(ctx)

	// 构建用户创建时间过滤条件
	userCreatedFilter := ""
	args := []interface{}{}
	if !userCreatedAt.IsZero() {
		userCreatedFilter = " AND sm.created_at >= ?"
	}

	var result *gorm.DB
	if tenantID != nil {
		baseArgs := []interface{}{ctxTenantID, userID, now, now, *tenantID}
		if !userCreatedAt.IsZero() {
			baseArgs = append(baseArgs, userCreatedAt)
		}
		args = baseArgs
		result = TenantDB(r.db, ctx).Exec(`
			INSERT INTO site_message_reads (id, tenant_id, message_id, user_id, read_at)
			SELECT gen_random_uuid(), ?, sm.id, ?, ? FROM site_messages sm
			WHERE (sm.expires_at IS NULL OR sm.expires_at > ?)
			AND (sm.target_tenant_id IS NULL OR sm.target_tenant_id = ?)
			AND NOT EXISTS (SELECT 1 FROM site_message_reads WHERE message_id = sm.id AND user_id = $2)`+userCreatedFilter+`
		`, args...)
	} else {
		baseArgs := []interface{}{ctxTenantID, userID, now, now}
		if !userCreatedAt.IsZero() {
			baseArgs = append(baseArgs, userCreatedAt)
		}
		args = baseArgs
		result = TenantDB(r.db, ctx).Exec(`
			INSERT INTO site_message_reads (id, tenant_id, message_id, user_id, read_at)
			SELECT gen_random_uuid(), ?, sm.id, ?, ? FROM site_messages sm
			WHERE (sm.expires_at IS NULL OR sm.expires_at > ?)
			AND NOT EXISTS (SELECT 1 FROM site_message_reads WHERE message_id = sm.id AND user_id = $2)`+userCreatedFilter+`
		`, args...)
	}

	return result.RowsAffected, result.Error
}

// Create 创建站内信
func (r *SiteMessageRepository) Create(ctx context.Context, msg *model.SiteMessage) error {
	// 自动设置 tenant_id
	if msg.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		msg.TenantID = &tenantID
	}
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

	tenantID := TenantIDFromContext(ctx)
	retentionDays := r.platformSettings.GetIntValue(ctx, "site_message.retention_days", 90)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, msg := range msgs {
			// 自动设置 tenant_id
			if msg.TenantID == nil {
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
	result := TenantDB(r.db, ctx).
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
