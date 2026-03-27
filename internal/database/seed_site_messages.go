package database

import (
	"context"
	"errors"
	"time"

	engagementmodel "github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SeedSiteMessages 插入站内信测试数据（15 条，覆盖 6 个分类）
func SeedSiteMessages() error {
	ctx := context.Background()
	messages := buildSeedSiteMessages(time.Now().AddDate(0, 0, 90))
	inserted, skipped, err := syncSiteMessages(ctx, messages)
	if err != nil {
		return err
	}
	logger.Info("站内信种子数据同步完成，新建 %d 条，跳过 %d 条", inserted, skipped)
	return nil
}

func syncSiteMessages(ctx context.Context, messages []engagementmodel.SiteMessage) (int, int, error) {
	inserted := 0
	skipped := 0

	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range messages {
			exists, err := seedSiteMessageExists(tx, messages[i])
			if err != nil {
				return err
			}
			if exists {
				skipped++
				continue
			}
			prepareSeedSiteMessage(&messages[i])
			if err := tx.Create(&messages[i]).Error; err != nil {
				return err
			}
			inserted++
		}
		return nil
	})
	return inserted, skipped, err
}

func seedSiteMessageExists(tx *gorm.DB, message engagementmodel.SiteMessage) (bool, error) {
	var existing engagementmodel.SiteMessage
	err := tx.
		Where("tenant_id IS NULL").
		Where("target_tenant_id IS NULL").
		Where("category = ? AND title = ?", message.Category, message.Title).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func prepareSeedSiteMessage(message *engagementmodel.SiteMessage) {
	if message.ID == uuid.Nil {
		message.ID = uuid.New()
	}
}
