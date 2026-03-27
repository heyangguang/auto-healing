package repository

import (
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"gorm.io/gorm"
)

type AuditLogListOptions = auditrepo.AuditLogListOptions
type AuditLogRepository = auditrepo.AuditLogRepository

func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return auditrepo.NewAuditLogRepository(db)
}
