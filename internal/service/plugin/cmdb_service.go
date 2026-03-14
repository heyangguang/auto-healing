package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CMDBService CMDB 服务
type CMDBService struct {
	cmdbRepo *repository.CMDBItemRepository
}

// NewCMDBService 创建 CMDB 服务
func NewCMDBService() *CMDBService {
	return &CMDBService{
		cmdbRepo: repository.NewCMDBItemRepository(),
	}
}

// GetCMDBItem 获取配置项
func (s *CMDBService) GetCMDBItem(ctx context.Context, id uuid.UUID) (*model.CMDBItem, error) {
	return s.cmdbRepo.GetByID(ctx, id)
}

// ListCMDBItems 获取配置项列表
func (s *CMDBService) ListCMDBItems(ctx context.Context, page, pageSize int, pluginID *uuid.UUID, itemType, status, environment, sourcePluginName string, search query.StringFilter, hasPlugin *bool, sortBy, sortOrder string, scopes ...func(*gorm.DB) *gorm.DB) ([]model.CMDBItem, int64, error) {
	return s.cmdbRepo.List(ctx, page, pageSize, pluginID, itemType, status, environment, sourcePluginName, search, hasPlugin, sortBy, sortOrder, scopes...)
}

// ListCMDBItemIDs 获取符合筛选条件的配置项 ID 列表（轻量接口）
func (s *CMDBService) ListCMDBItemIDs(ctx context.Context, pluginID *uuid.UUID, itemType, status, environment, sourcePluginName string, hasPlugin *bool) ([]repository.CMDBItemBasic, int64, error) {
	return s.cmdbRepo.ListIDs(ctx, pluginID, itemType, status, environment, sourcePluginName, hasPlugin)
}

// GetCMDBStats 获取统计信息
func (s *CMDBService) GetCMDBStats(ctx context.Context) (map[string]interface{}, error) {
	return s.cmdbRepo.GetStats(ctx)
}

// EnterMaintenance 进入维护模式
func (s *CMDBService) EnterMaintenance(ctx context.Context, id uuid.UUID, reason string, endAt *time.Time, operator string) error {
	// 获取配置项信息
	item, err := s.cmdbRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 检查状态：offline 不能进入维护
	if item.Status == "offline" {
		return fmt.Errorf("已下线的配置项不能进入维护模式")
	}

	// 进入维护
	if err := s.cmdbRepo.EnterMaintenance(ctx, id, reason, endAt); err != nil {
		return err
	}

	// 记录日志
	log := &model.CMDBMaintenanceLog{
		CMDBItemID:     id,
		CMDBItemName:   item.Name,
		Action:         "enter",
		Reason:         reason,
		ScheduledEndAt: endAt,
		Operator:       operator,
	}
	return s.cmdbRepo.CreateMaintenanceLog(ctx, log)
}

// ExitMaintenance 退出维护模式
func (s *CMDBService) ExitMaintenance(ctx context.Context, id uuid.UUID, exitType, operator string) error {
	// 获取配置项信息
	item, err := s.cmdbRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 退出维护
	if err := s.cmdbRepo.ExitMaintenance(ctx, id); err != nil {
		return err
	}

	// 记录日志
	now := time.Now()
	log := &model.CMDBMaintenanceLog{
		CMDBItemID:   id,
		CMDBItemName: item.Name,
		Action:       "exit",
		ActualEndAt:  &now,
		ExitType:     exitType,
		Operator:     operator,
	}
	return s.cmdbRepo.CreateMaintenanceLog(ctx, log)
}

// GetMaintenanceLogs 获取维护日志
func (s *CMDBService) GetMaintenanceLogs(ctx context.Context, id uuid.UUID, page, pageSize int) ([]model.CMDBMaintenanceLog, int64, error) {
	return s.cmdbRepo.ListMaintenanceLogs(ctx, id, page, pageSize)
}

// CheckExpiredMaintenance 检查并恢复到期的维护（跨租户）
func (s *CMDBService) CheckExpiredMaintenance(ctx context.Context) (int, error) {
	items, err := s.cmdbRepo.GetExpiredMaintenanceItems(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, item := range items {
		// 注入该配置项所属租户的上下文，确保 ExitMaintenance 在正确租户范围内操作
		itemCtx := ctx
		if item.TenantID != nil {
			itemCtx = repository.WithTenantID(ctx, *item.TenantID)
		}
		if err := s.ExitMaintenance(itemCtx, item.ID, "auto", "system"); err == nil {
			count++
		}
	}
	return count, nil
}
