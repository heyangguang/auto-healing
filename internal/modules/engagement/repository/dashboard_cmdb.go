package repository

import (
	"context"
	"time"

	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CMDBSection struct {
	Total             int64             `json:"total"`
	Active            int64             `json:"active"`
	Maintenance       int64             `json:"maintenance"`
	Offline           int64             `json:"offline"`
	ActiveRate        float64           `json:"active_rate"`
	ByStatus          []StatusCount     `json:"by_status"`
	ByEnvironment     []StatusCount     `json:"by_environment"`
	ByType            []StatusCount     `json:"by_type"`
	ByOS              []StatusCount     `json:"by_os"`
	ByDepartment      []StatusCount     `json:"by_department"`
	ByManufacturer    []StatusCount     `json:"by_manufacturer"`
	RecentMaintenance []MaintenanceItem `json:"recent_maintenance"`
	OfflineAssets     []AssetItem       `json:"offline_assets"`
}

type MaintenanceItem struct {
	ID           uuid.UUID `json:"id"`
	CMDBItemName string    `json:"cmdb_item_name"`
	Action       string    `json:"action"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

type AssetItem struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	IPAddress   string    `json:"ip_address"`
	Environment string    `json:"environment"`
}

func (r *DashboardRepository) GetCMDBSection(ctx context.Context) (*CMDBSection, error) {
	section := &CMDBSection{
		ByStatus:       []StatusCount{},
		ByEnvironment:  []StatusCount{},
		ByType:         []StatusCount{},
		ByOS:           []StatusCount{},
		ByDepartment:   []StatusCount{},
		ByManufacturer: []StatusCount{},
	}
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }

	if err := countModel(newDB(), &projection.CMDBItem{}, &section.Total); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "active"), &projection.CMDBItem{}, &section.Active); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "maintenance"), &projection.CMDBItem{}, &section.Maintenance); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "offline"), &projection.CMDBItem{}, &section.Offline); err != nil {
		return nil, err
	}
	if section.Total > 0 {
		section.ActiveRate = float64(section.Active) / float64(section.Total) * 100
	}

	if err := scanStatusCounts(newDB(), &projection.CMDBItem{}, "status", &section.ByStatus); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.CMDBItem{}, "environment", &section.ByEnvironment); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.CMDBItem{}, "type", &section.ByType); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.CMDBItem{}, "os", &section.ByOS); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.CMDBItem{}, "department", &section.ByDepartment); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.CMDBItem{}, "manufacturer", &section.ByManufacturer); err != nil {
		return nil, err
	}
	recent, err := listRecentMaintenance(newDB().Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentMaintenance = recent
	offline, err := listOfflineAssets(newDB().Where("status = ?", "offline").Order("updated_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.OfflineAssets = offline
	return section, nil
}

func listRecentMaintenance(query *gorm.DB) ([]MaintenanceItem, error) {
	var logs []projection.CMDBMaintenanceLog
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	items := make([]MaintenanceItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, MaintenanceItem{
			ID:           log.ID,
			CMDBItemName: log.CMDBItemName,
			Action:       log.Action,
			Reason:       log.Reason,
			CreatedAt:    log.CreatedAt,
		})
	}
	return items, nil
}

func listOfflineAssets(query *gorm.DB) ([]AssetItem, error) {
	var assets []projection.CMDBItem
	if err := query.Find(&assets).Error; err != nil {
		return nil, err
	}
	items := make([]AssetItem, 0, len(assets))
	for _, asset := range assets {
		items = append(items, AssetItem{
			ID:          asset.ID,
			Name:        asset.Name,
			Type:        asset.Type,
			IPAddress:   asset.IPAddress,
			Environment: asset.Environment,
		})
	}
	return items, nil
}
