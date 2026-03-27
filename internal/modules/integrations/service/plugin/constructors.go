package plugin

import (
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"gorm.io/gorm"
)

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		PluginRepo:   integrationrepo.NewPluginRepositoryWithDB(db),
		SyncLogRepo:  integrationrepo.NewPluginSyncLogRepositoryWithDB(db),
		CMDBRepo:     cmdbrepo.NewCMDBItemRepositoryWithDB(db),
		IncidentRepo: incidentrepo.NewIncidentRepositoryWithDB(db),
		HTTPClient:   NewHTTPClient(),
		Lifecycle:    newAsyncLifecycle(),
	}
}

func DefaultIncidentServiceDepsWithDB(db *gorm.DB) IncidentServiceDeps {
	return IncidentServiceDeps{
		IncidentRepo: incidentrepo.NewIncidentRepositoryWithDB(db),
		PluginRepo:   integrationrepo.NewPluginRepositoryWithDB(db),
		HTTPClient:   NewHTTPClient(),
	}
}

func DefaultCMDBServiceDepsWithDB(db *gorm.DB) CMDBServiceDeps {
	return CMDBServiceDeps{
		CMDBRepo: cmdbrepo.NewCMDBItemRepositoryWithDB(db),
	}
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}

func NewIncidentServiceWithDB(db *gorm.DB) *IncidentService {
	return NewIncidentServiceWithDeps(DefaultIncidentServiceDepsWithDB(db))
}

func NewCMDBServiceWithDB(db *gorm.DB) *CMDBService {
	return NewCMDBServiceWithDeps(DefaultCMDBServiceDepsWithDB(db))
}
