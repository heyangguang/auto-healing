package incident

import "github.com/company/auto-healing/internal/database"

// NewIncidentRepository 保留零参兼容入口，生产主路径请显式传 db。
func NewIncidentRepository() *IncidentRepository {
	return NewIncidentRepositoryWithDB(database.DB)
}
