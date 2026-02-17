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
	db *gorm.DB
}

// NewSiteMessageRepository 创建站内信仓库
func NewSiteMessageRepository() *SiteMessageRepository {
	return &SiteMessageRepository{db: database.DB}
}

// List 分页查询站内信（带已读状态），支持 keyword 和 category 筛选
func (r *SiteMessageRepository) List(ctx context.Context, userID uuid.UUID, page, pageSize int, keyword, category string) ([]model.SiteMessageWithReadStatus, int64, error) {
	var total int64
	var results []model.SiteMessageWithReadStatus

	// 基础查询：未过期的消息
	baseQuery := r.db.WithContext(ctx).Table("site_messages AS sm").
		Where("(sm.expires_at IS NULL OR sm.expires_at > ?)", time.Now())

	// 筛选条件
	if keyword != "" {
		baseQuery = baseQuery.Where("sm.title ILIKE ?", "%"+keyword+"%")
	}
	if category != "" {
		baseQuery = baseQuery.Where("sm.category = ?", category)
	}

	// 先计算总数
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询带已读状态的列表
	err := baseQuery.
		Select(`sm.id, sm.category, sm.title, sm.content, sm.created_at, sm.expires_at,
			CASE WHEN smr.id IS NOT NULL THEN true ELSE false END AS is_read`).
		Joins("LEFT JOIN site_message_reads AS smr ON smr.message_id = sm.id AND smr.user_id = ?", userID).
		Order("sm.created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&results).Error

	return results, total, err
}

// GetUnreadCount 获取当前用户的未读站内信数量（轻量查询）
func (r *SiteMessageRepository) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("site_messages AS sm").
		Where("(sm.expires_at IS NULL OR sm.expires_at > ?)", time.Now()).
		Where("NOT EXISTS (SELECT 1 FROM site_message_reads WHERE message_id = sm.id AND user_id = ?)", userID).
		Count(&count).Error
	return count, err
}

// MarkRead 批量标记已读（UPSERT，已读的跳过）
func (r *SiteMessageRepository) MarkRead(ctx context.Context, userID uuid.UUID, messageIDs []uuid.UUID) error {
	if len(messageIDs) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, msgID := range messageIDs {
			read := model.SiteMessageRead{
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
func (r *SiteMessageRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	now := time.Now()
	// INSERT INTO site_message_reads (message_id, user_id, read_at)
	// SELECT sm.id, ?, ? FROM site_messages sm
	// WHERE (sm.expires_at IS NULL OR sm.expires_at > ?)
	// AND NOT EXISTS (SELECT 1 FROM site_message_reads WHERE message_id = sm.id AND user_id = ?)
	result := r.db.WithContext(ctx).Exec(`
		INSERT INTO site_message_reads (id, message_id, user_id, read_at)
		SELECT gen_random_uuid(), sm.id, ?, ? FROM site_messages sm
		WHERE (sm.expires_at IS NULL OR sm.expires_at > ?)
		AND NOT EXISTS (SELECT 1 FROM site_message_reads WHERE message_id = sm.id AND user_id = ?)
	`, userID, now, now, userID)

	return result.RowsAffected, result.Error
}

// Create 创建站内信
func (r *SiteMessageRepository) Create(ctx context.Context, msg *model.SiteMessage) error {
	// 如果没有设置过期时间，按当前配置计算
	if msg.ExpiresAt == nil {
		settings, err := r.GetSettings(ctx)
		if err == nil && settings.RetentionDays > 0 {
			expiresAt := time.Now().AddDate(0, 0, settings.RetentionDays)
			msg.ExpiresAt = &expiresAt
		}
	}
	return r.db.WithContext(ctx).Create(msg).Error
}

// GetSettings 获取站内信设置
func (r *SiteMessageRepository) GetSettings(ctx context.Context) (*model.SiteMessageSettings, error) {
	var settings model.SiteMessageSettings
	err := r.db.WithContext(ctx).First(&settings).Error
	if err != nil {
		// 如果没有设置行，返回默认值
		if err == gorm.ErrRecordNotFound {
			return &model.SiteMessageSettings{RetentionDays: 90}, nil
		}
		return nil, err
	}
	return &settings, nil
}

// UpdateSettings 更新站内信设置
func (r *SiteMessageRepository) UpdateSettings(ctx context.Context, retentionDays int) (*model.SiteMessageSettings, error) {
	var settings model.SiteMessageSettings
	err := r.db.WithContext(ctx).First(&settings).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 不存在则创建
			settings = model.SiteMessageSettings{
				RetentionDays: retentionDays,
				UpdatedAt:     time.Now(),
			}
			if err := r.db.WithContext(ctx).Create(&settings).Error; err != nil {
				return nil, err
			}
			return &settings, nil
		}
		return nil, err
	}

	// 更新已有设置
	settings.RetentionDays = retentionDays
	settings.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Save(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
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
