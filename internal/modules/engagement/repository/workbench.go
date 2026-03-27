package repository

import (
	"context"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"time"

	"gorm.io/gorm"
)

// appStartTime 应用启动时间（用于计算 uptime）
var appStartTime = time.Now()

// WorkbenchRepository 工作台仓库
type WorkbenchRepository struct {
	db *gorm.DB
}

// NewWorkbenchRepository 创建工作台仓库
func NewWorkbenchRepository(db *gorm.DB) *WorkbenchRepository {
	return &WorkbenchRepository{db: db}
}

func (r *WorkbenchRepository) tenantDB(ctx context.Context) *gorm.DB {
	return platformrepo.TenantDB(r.db, ctx)
}

// FavoriteItem 收藏项
type FavoriteItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Path  string `json:"path"`
}
