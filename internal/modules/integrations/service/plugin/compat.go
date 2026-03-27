package plugin

import (
	"github.com/company/auto-healing/internal/database"
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
