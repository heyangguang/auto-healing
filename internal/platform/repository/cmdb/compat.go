package cmdb

import "github.com/company/auto-healing/internal/database"

// NewCMDBItemRepository 保留零参兼容入口，生产主路径请显式传 db。
func NewCMDBItemRepository() *CMDBItemRepository {
	return NewCMDBItemRepositoryWithDB(database.DB)
}
