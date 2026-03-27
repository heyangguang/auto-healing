package repository

import (
	"context"
	"time"

	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IncidentSection struct {
	Total             int64         `json:"total"`
	Today             int64         `json:"today"`
	ThisWeek          int64         `json:"this_week"`
	Unscanned         int64         `json:"unscanned"`
	HealingRate       float64       `json:"healing_rate"`
	ByHealingStatus   []StatusCount `json:"by_healing_status"`
	BySeverity        []StatusCount `json:"by_severity"`
	ByCategory        []StatusCount `json:"by_category"`
	ByStatus          []StatusCount `json:"by_status"`
	BySource          []StatusCount `json:"by_source"`
	Trend7d           []TrendPoint  `json:"trend_7d"`
	Trend30d          []TrendPoint  `json:"trend_30d"`
	RecentIncidents   []RecentItem  `json:"recent_incidents"`
	CriticalIncidents []RecentItem  `json:"critical_incidents"`
}

type RecentItem struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *DashboardRepository) GetIncidentSection(ctx context.Context) (*IncidentSection, error) {
	section := &IncidentSection{}
	db := r.tenantDB(ctx)
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, 0, -30)

	if err := countModel(db, &projection.Incident{}, &section.Total); err != nil {
		return nil, err
	}
	if err := countModel(db.Where("created_at >= ?", now.Truncate(24*time.Hour)), &projection.Incident{}, &section.Today); err != nil {
		return nil, err
	}
	if err := countModel(db.Where("created_at >= ?", weekAgo), &projection.Incident{}, &section.ThisWeek); err != nil {
		return nil, err
	}
	if err := countModel(db.Where("scanned = ?", false), &projection.Incident{}, &section.Unscanned); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(db, &projection.Incident{}, "healing_status", &section.ByHealingStatus); err != nil {
		return nil, err
	}
	section.HealingRate = calculateHealingRate(section.ByHealingStatus)
	if err := scanStatusCounts(db, &projection.Incident{}, "severity", &section.BySeverity); err != nil {
		return nil, err
	}
	if err := db.Model(&projection.Incident{}).
		Select("COALESCE(NULLIF(category, ''), 'unknown') as status, count(*) as count").
		Group("COALESCE(NULLIF(category, ''), 'unknown')").
		Order("count DESC").
		Scan(&section.ByCategory).Error; err != nil {
		return nil, err
	}
	if err := scanStatusCounts(db, &projection.Incident{}, "status", &section.ByStatus); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(db, &projection.Incident{}, "source_plugin_name", &section.BySource); err != nil {
		return nil, err
	}
	if err := scanTrendPoints(db, &projection.Incident{}, "created_at", weekAgo, &section.Trend7d); err != nil {
		return nil, err
	}
	if err := scanTrendPoints(db, &projection.Incident{}, "created_at", monthAgo, &section.Trend30d); err != nil {
		return nil, err
	}
	recent, err := listRecentIncidents(db.Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentIncidents = recent
	critical, err := listRecentIncidents(db.Where("severity = ?", "critical").Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.CriticalIncidents = critical
	return section, nil
}

func calculateHealingRate(statuses []StatusCount) float64 {
	var healed, failed int64
	for _, item := range statuses {
		switch item.Status {
		case "healed":
			healed = item.Count
		case "failed":
			failed = item.Count
		}
	}
	if healed+failed == 0 {
		return 0
	}
	return float64(healed) / float64(healed+failed) * 100
}

func listRecentIncidents(query *gorm.DB) ([]RecentItem, error) {
	var incidents []projection.Incident
	if err := query.Find(&incidents).Error; err != nil {
		return nil, err
	}
	items := make([]RecentItem, 0, len(incidents))
	for _, incident := range incidents {
		items = append(items, RecentItem{
			ID:        incident.ID,
			Title:     incident.Title,
			Status:    incident.HealingStatus,
			CreatedAt: incident.CreatedAt,
		})
	}
	return items, nil
}
