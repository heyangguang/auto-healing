package playbook

import (
	"github.com/company/auto-healing/internal/database"
	"gorm.io/gorm"
)

func DefaultServiceDeps() ServiceDeps {
	return DefaultServiceDepsWithDB(database.DB)
}

// NewService 保留兼容零参构造，生产路径应使用显式 deps。
func NewService() *Service {
	return NewServiceWithDB(database.DB)
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db))
}
