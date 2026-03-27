package repository

import (
	"context"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MarkRead 批量标记已读（UPSERT，已读的跳过）
func (r *SiteMessageRepository) MarkRead(ctx context.Context, userID uuid.UUID, messageIDs []uuid.UUID) error {
	if len(messageIDs) == 0 {
		return nil
	}
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return err
	}
	existingIDs, err := r.existingMessageIDs(ctx, tenantID, messageIDs)
	if err != nil || len(existingIDs) == 0 {
		return err
	}
	return r.insertSiteMessageReads(ctx, tenantID, userID, existingIDs)
}

func (r *SiteMessageRepository) existingMessageIDs(ctx context.Context, tenantID uuid.UUID, messageIDs []uuid.UUID) ([]uuid.UUID, error) {
	var existingIDs []uuid.UUID
	err := r.db.WithContext(ctx).
		Model(&model.SiteMessage{}).
		Where("id IN ?", messageIDs).
		Where("target_tenant_id IS NULL OR target_tenant_id = ?", tenantID).
		Pluck("id", &existingIDs).Error
	return existingIDs, err
}

func (r *SiteMessageRepository) insertSiteMessageReads(ctx context.Context, tenantID, userID uuid.UUID, messageIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, msgID := range messageIDs {
			read := model.SiteMessageRead{
				TenantID:  &tenantID,
				MessageID: msgID,
				UserID:    userID,
				ReadAt:    time.Now(),
			}
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
func (r *SiteMessageRepository) MarkAllRead(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, userCreatedAt time.Time) (int64, error) {
	now := time.Now()
	ctxTenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return 0, err
	}
	result := r.markAllReadQuery(ctx, ctxTenantID, userID, tenantID, userCreatedAt, now)
	return result.RowsAffected, result.Error
}

func (r *SiteMessageRepository) markAllReadQuery(ctx context.Context, ctxTenantID, userID uuid.UUID, tenantID *uuid.UUID, userCreatedAt, now time.Time) *gorm.DB {
	userCreatedFilter := ""
	if !userCreatedAt.IsZero() {
		userCreatedFilter = " AND sm.created_at >= ?"
	}
	if tenantID != nil {
		baseArgs := []interface{}{ctxTenantID, userID, now, now, *tenantID}
		if !userCreatedAt.IsZero() {
			baseArgs = append(baseArgs, userCreatedAt)
		}
		return r.db.WithContext(ctx).Exec(`
			INSERT INTO site_message_reads (id, tenant_id, message_id, user_id, read_at)
			SELECT gen_random_uuid(), ?, sm.id, ?, ? FROM site_messages sm
			WHERE (sm.expires_at IS NULL OR sm.expires_at > ?)
			AND (sm.target_tenant_id IS NULL OR sm.target_tenant_id = ?)
			AND NOT EXISTS (SELECT 1 FROM site_message_reads WHERE tenant_id = $1 AND message_id = sm.id AND user_id = $2)`+userCreatedFilter+`
		`, baseArgs...)
	}
	baseArgs := []interface{}{ctxTenantID, userID, now, now}
	if !userCreatedAt.IsZero() {
		baseArgs = append(baseArgs, userCreatedAt)
	}
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO site_message_reads (id, tenant_id, message_id, user_id, read_at)
		SELECT gen_random_uuid(), ?, sm.id, ?, ? FROM site_messages sm
		WHERE (sm.expires_at IS NULL OR sm.expires_at > ?)
		AND NOT EXISTS (SELECT 1 FROM site_message_reads WHERE tenant_id = $1 AND message_id = sm.id AND user_id = $2)`+userCreatedFilter+`
	`, baseArgs...)
}

// Create 创建站内信
func (r *SiteMessageRepository) Create(ctx context.Context, msg *model.SiteMessage) error {
	r.prepareSiteMessageCreate(ctx, msg)
	return r.db.WithContext(ctx).Create(msg).Error
}

// CreateBatch 在一个事务中批量创建多条站内信
func (r *SiteMessageRepository) CreateBatch(ctx context.Context, msgs []*model.SiteMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	tenantID, hasTenant := platformrepo.TenantIDFromContextOK(ctx)
	retentionDays := r.platformSettings.GetIntValue(ctx, "site_message.retention_days", 90)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, msg := range msgs {
			prepareSiteMessageBatchCreate(msg, hasTenant, tenantID, retentionDays)
			if err := tx.Create(msg).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SiteMessageRepository) prepareSiteMessageCreate(ctx context.Context, msg *model.SiteMessage) {
	if msg.TenantID == nil {
		if tenantID, ok := platformrepo.TenantIDFromContextOK(ctx); ok {
			msg.TenantID = &tenantID
		}
	}
	if msg.ExpiresAt != nil {
		return
	}
	retentionDays := r.platformSettings.GetIntValue(ctx, "site_message.retention_days", 90)
	if retentionDays > 0 {
		expiresAt := time.Now().AddDate(0, 0, retentionDays)
		msg.ExpiresAt = &expiresAt
	}
}

func prepareSiteMessageBatchCreate(msg *model.SiteMessage, hasTenant bool, tenantID uuid.UUID, retentionDays int) {
	if msg.TenantID == nil && hasTenant {
		tid := tenantID
		msg.TenantID = &tid
	}
	if msg.ExpiresAt == nil && retentionDays > 0 {
		expiresAt := time.Now().AddDate(0, 0, retentionDays)
		msg.ExpiresAt = &expiresAt
	}
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
