package plugin

import (
	"github.com/company/auto-healing/internal/database"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"gorm.io/gorm"
)

func DefaultServiceDeps() ServiceDeps {
	return DefaultServiceDepsWithDB(database.DB)
}

func DefaultIncidentServiceDeps() IncidentServiceDeps {
	return DefaultIncidentServiceDepsWithDB(database.DB)
}

func DefaultCMDBServiceDeps() CMDBServiceDeps {
	return DefaultCMDBServiceDepsWithDB(database.DB)
}

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

// NewService 保留兼容零参构造，生产路径应使用显式 deps。
func NewService() *Service {
	return NewServiceWithDB(database.DB)
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}

// NewIncidentService 保留兼容零参构造，生产路径应使用显式 deps。
func NewIncidentService() *IncidentService {
	return NewIncidentServiceWithDB(database.DB)
}

func NewIncidentServiceWithDB(db *gorm.DB) *IncidentService {
	return NewIncidentServiceWithDeps(DefaultIncidentServiceDepsWithDB(db))
}

// NewCMDBService 保留兼容零参构造，生产路径应使用显式 deps。
func NewCMDBService() *CMDBService {
	return NewCMDBServiceWithDB(database.DB)
}

func NewCMDBServiceWithDB(db *gorm.DB) *CMDBService {
	return NewCMDBServiceWithDeps(DefaultCMDBServiceDepsWithDB(db))
}
