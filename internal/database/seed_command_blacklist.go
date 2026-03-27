package database

import (
	"context"
	"fmt"
	"time"

	opsmodel "github.com/company/auto-healing/internal/modules/ops/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SeedCommandBlacklist 种子化高危指令黑名单（增量：只插入不存在的）
func SeedCommandBlacklist() error {
	ctx := context.Background()
	now := time.Now()
	inserted, err := syncCommandBlacklistSeeds(ctx, now, commandBlacklistSeeds)
	if err != nil {
		return err
	}

	if inserted > 0 {
		logger.Info("高危指令黑名单种子数据: 新增 %d 条", inserted)
		return nil
	}
	logger.Info("高危指令黑名单种子数据已是最新，无需新增")
	return nil
}

func syncCommandBlacklistSeeds(ctx context.Context, now time.Time, seeds []opsmodel.CommandBlacklist) (int, error) {
	inserted := 0
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, seed := range seeds {
			ok, err := seedCommandBlacklistEntry(ctx, tx, now, seed)
			if err != nil {
				return fmt.Errorf("同步黑名单种子 %s 失败: %w", seed.Name, err)
			}
			if ok {
				inserted++
			}
		}
		return nil
	})
	return inserted, err
}

func seedCommandBlacklistEntry(ctx context.Context, tx *gorm.DB, now time.Time, seed opsmodel.CommandBlacklist) (bool, error) {
	var count int64
	if err := tx.WithContext(ctx).Model(&opsmodel.CommandBlacklist{}).
		Where("name = ? AND pattern = ?", seed.Name, seed.Pattern).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}

	if seed.ID == uuid.Nil {
		seed.ID = uuid.New()
	}
	seed.CreatedAt = now
	seed.UpdatedAt = now
	return true, tx.WithContext(ctx).Create(&seed).Error
}
