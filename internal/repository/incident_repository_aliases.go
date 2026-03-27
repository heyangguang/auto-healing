package repository

import (
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"gorm.io/gorm"
)

type IncidentRepository = incidentrepo.IncidentRepository
type IncidentStats = incidentrepo.IncidentStats
type IncidentSyncOptions = incidentrepo.IncidentSyncOptions

var ErrIncidentNotFound = incidentrepo.ErrIncidentNotFound

func NewIncidentRepository() *IncidentRepository {
	return incidentrepo.NewIncidentRepository()
}

func NewIncidentRepositoryWithDB(db *gorm.DB) *IncidentRepository {
	return incidentrepo.NewIncidentRepositoryWithDB(db)
}
