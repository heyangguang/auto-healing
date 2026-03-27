package repository

import "github.com/company/auto-healing/internal/database"

// NewSecretsSourceRepository 保留兼容零参构造，生产路径应使用显式 deps。
func NewSecretsSourceRepository() *SecretsSourceRepository {
	return NewSecretsSourceRepositoryWithDB(database.DB)
}
