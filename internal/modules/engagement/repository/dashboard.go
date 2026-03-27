package repository

import (
	"context"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DashboardRepository Dashboard 仓库
type DashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepositoryWithDB(db *gorm.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

func (r *DashboardRepository) tenantDB(ctx context.Context) *gorm.DB {
	return platformrepo.TenantDB(r.db, ctx)
}

// GetConfigByUserID 获取用户配置
func (r *DashboardRepository) GetConfigByUserID(ctx context.Context, userID uuid.UUID) (*model.DashboardConfig, error) {
	var config model.DashboardConfig
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if tenantID, ok := platformrepo.TenantIDFromContextOK(ctx); ok {
		query = query.Where("tenant_id = ?", tenantID)
	} else {
		query = query.Where("tenant_id IS NULL")
	}
	err := query.First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// UpsertConfig 创建或更新用户配置（使用 ON CONFLICT DO UPDATE 保证原子性）
func (r *DashboardRepository) UpsertConfig(ctx context.Context, userID uuid.UUID, configData model.JSON) error {
	config := model.DashboardConfig{UserID: userID, Config: configData}
	if tenantID, ok := platformrepo.TenantIDFromContextOK(ctx); ok {
		config.TenantID = &tenantID
		return r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "tenant_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"config":     configData,
				"updated_at": time.Now(),
			}),
		}).Create(&config).Error
	}

	result := r.db.WithContext(ctx).Model(&model.DashboardConfig{}).
		Where("user_id = ? AND tenant_id IS NULL", userID).
		Updates(map[string]any{
			"config":     configData,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.db.WithContext(ctx).Create(&config).Error
	}
	return nil
}

type StatusCount struct {
	Status string `json:"status" gorm:"column:status"`
	Count  int64  `json:"count" gorm:"column:count"`
}

type TrendPoint struct {
	Date  string `json:"date" gorm:"column:date"`
	Count int64  `json:"count" gorm:"column:count"`
}

type RankItem struct {
	Name  string `json:"name" gorm:"column:name"`
	Count int64  `json:"count" gorm:"column:count"`
}

func countModel(query *gorm.DB, model any, dest *int64) error {
	return query.Model(model).Count(dest).Error
}

func scanStatusCounts(query *gorm.DB, model any, column string, dest *[]StatusCount) error {
	return query.Model(model).Select(column + " as status, count(*) as count").Group(column).Scan(dest).Error
}

func scanTrendPoints(query *gorm.DB, model any, timeColumn string, since time.Time, dest *[]TrendPoint) error {
	return query.Model(model).
		Select("DATE("+timeColumn+") as date, count(*) as count").
		Where(timeColumn+" >= ?", since).
		Group("DATE(" + timeColumn + ")").
		Order("date").
		Scan(dest).Error
}
