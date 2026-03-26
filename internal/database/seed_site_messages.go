package database

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"gorm.io/gorm"
)

// SeedSiteMessages 插入站内信测试数据（15 条，覆盖 6 个分类）
func SeedSiteMessages() error {
	ctx := context.Background()

	count, err := countSiteMessages(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		logger.Info("站内信已有 %d 条数据，跳过种子数据插入", count)
		return nil
	}

	logger.Info("插入站内信种子数据...")
	messages := buildSeedSiteMessages(time.Now().AddDate(0, 0, 90))
	return insertSeedSiteMessages(ctx, messages)
}

func countSiteMessages(ctx context.Context) (int64, error) {
	var count int64
	err := DB.WithContext(ctx).Model(&model.SiteMessage{}).Count(&count).Error
	return count, err
}

func insertSeedSiteMessages(ctx context.Context, messages []model.SiteMessage) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range messages {
			if err := tx.Create(&messages[i]).Error; err != nil {
				return err
			}
		}
		logger.Info("站内信种子数据插入完成，共 %d 条", len(messages))
		return nil
	})
}
