package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
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

	countModel(db, &model.Incident{}, &section.Total)
	countModel(db.Where("created_at >= ?", now.Truncate(24*time.Hour)), &model.Incident{}, &section.Today)
	countModel(db.Where("created_at >= ?", weekAgo), &model.Incident{}, &section.ThisWeek)
	countModel(db.Where("scanned = ?", false), &model.Incident{}, &section.Unscanned)

	scanStatusCounts(db, &model.Incident{}, "healing_status", &section.ByHealingStatus)
	section.HealingRate = calculateHealingRate(section.ByHealingStatus)
	scanStatusCounts(db, &model.Incident{}, "severity", &section.BySeverity)
	db.Model(&model.Incident{}).
		Select("COALESCE(NULLIF(category, ''), 'unknown') as status, count(*) as count").
		Group("COALESCE(NULLIF(category, ''), 'unknown')").
		Order("count DESC").
		Scan(&section.ByCategory)
	scanStatusCounts(db, &model.Incident{}, "status", &section.ByStatus)
	scanStatusCounts(db, &model.Incident{}, "source_plugin_name", &section.BySource)
	scanTrendPoints(db, &model.Incident{}, "created_at", weekAgo, &section.Trend7d)
	scanTrendPoints(db, &model.Incident{}, "created_at", monthAgo, &section.Trend30d)

	section.RecentIncidents = listRecentIncidents(db.Order("created_at DESC").Limit(10))
	section.CriticalIncidents = listRecentIncidents(db.Where("severity = ?", "critical").Order("created_at DESC").Limit(10))
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

func listRecentIncidents(query *gorm.DB) []RecentItem {
	var incidents []model.Incident
	query.Find(&incidents)
	items := make([]RecentItem, 0, len(incidents))
	for _, incident := range incidents {
		items = append(items, RecentItem{
			ID:        incident.ID,
			Title:     incident.Title,
			Status:    incident.HealingStatus,
			CreatedAt: incident.CreatedAt,
		})
	}
	return items
}
