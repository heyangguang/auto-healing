package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AnnouncementItem 公告项
type AnnouncementItem struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// GetAnnouncements 获取系统公告列表
func (r *WorkbenchRepository) GetAnnouncements(ctx context.Context, limit int, userCreatedAt time.Time) ([]AnnouncementItem, error) {
	if limit <= 0 {
		limit = 5
	}

	var messages []model.SiteMessage
	err := r.workbenchAnnouncementQuery(ctx, userCreatedAt).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}

	items := make([]AnnouncementItem, 0, len(messages))
	for _, message := range messages {
		items = append(items, AnnouncementItem{
			ID:        message.ID,
			Title:     message.Title,
			Content:   message.Content,
			CreatedAt: message.CreatedAt,
		})
	}
	return items, nil
}

func (r *WorkbenchRepository) workbenchAnnouncementQuery(ctx context.Context, userCreatedAt time.Time) *gorm.DB {
	query := r.db.WithContext(ctx).
		Model(&model.SiteMessage{}).
		Where("category = ?", model.SiteMessageCategoryAnnouncement).
		Where("(expires_at IS NULL OR expires_at > ?)", time.Now()).
		Where("(target_tenant_id IS NULL OR target_tenant_id = ?)", TenantIDFromContext(ctx))

	if !userCreatedAt.IsZero() {
		query = query.Where("created_at >= ?", userCreatedAt)
	}
	return query
}
