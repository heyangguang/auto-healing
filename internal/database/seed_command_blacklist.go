package database

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// SeedCommandBlacklist 种子化高危指令黑名单（增量：只插入不存在的）
func SeedCommandBlacklist() error {
	ctx := context.Background()
	now := time.Now()
	inserted := 0

	for _, seed := range commandBlacklistSeeds {
		ok, err := seedCommandBlacklistEntry(ctx, now, &seed)
		if err != nil {
			logger.Warn("插入黑名单种子数据失败: %s (%v)", seed.Name, err)
			continue
		}
		if ok {
			inserted++
		}
	}

	if inserted > 0 {
		logger.Info("高危指令黑名单种子数据: 新增 %d 条", inserted)
	}
	return nil
}

func seedCommandBlacklistEntry(ctx context.Context, now time.Time, seed *model.CommandBlacklist) (bool, error) {
	var count int64
	if err := DB.WithContext(ctx).Model(&model.CommandBlacklist{}).
		Where("name = ? AND pattern = ?", seed.Name, seed.Pattern).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}

	seed.ID = uuid.New()
	seed.CreatedAt = now
	seed.UpdatedAt = now
	return true, DB.WithContext(ctx).Create(seed).Error
}
