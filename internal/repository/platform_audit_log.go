package repository

import (
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"gorm.io/gorm"
)

type PlatformAuditLogRepository = auditrepo.PlatformAuditLogRepository
type PlatformAuditListOptions = auditrepo.PlatformAuditListOptions
type PlatformAuditStats = auditrepo.PlatformAuditStats

func NewPlatformAuditLogRepository() *PlatformAuditLogRepository {
	return auditrepo.NewPlatformAuditLogRepository()
}

func NewPlatformAuditLogRepositoryWithDB(db *gorm.DB) *PlatformAuditLogRepository {
	return auditrepo.NewPlatformAuditLogRepositoryWithDB(db)
}
