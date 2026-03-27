package audit

import "github.com/company/auto-healing/internal/database"

// NewPlatformAuditLogRepository 保留零参兼容入口，生产主路径请显式传 db。
func NewPlatformAuditLogRepository() *PlatformAuditLogRepository {
	return NewPlatformAuditLogRepositoryWithDB(database.DB)
}
