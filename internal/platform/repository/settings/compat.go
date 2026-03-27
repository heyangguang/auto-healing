package repository

import "github.com/company/auto-healing/internal/database"

// NewPlatformSettingsRepository 保留零参兼容入口，生产主路径请显式传 db。
func NewPlatformSettingsRepository() *PlatformSettingsRepository {
	return NewPlatformSettingsRepositoryWithDB(database.DB)
}
