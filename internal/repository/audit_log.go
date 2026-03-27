package repository

import (
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"gorm.io/gorm"
)

const (
	RiskLevelLow      = auditrepo.RiskLevelLow
	RiskLevelMedium   = auditrepo.RiskLevelMedium
	RiskLevelHigh     = auditrepo.RiskLevelHigh
	RiskLevelCritical = auditrepo.RiskLevelCritical
)

type AuditLogListOptions = auditrepo.AuditLogListOptions
type AuditLogRepository = auditrepo.AuditLogRepository
type AuditStats = auditrepo.AuditStats
type ActionStat = auditrepo.ActionStat
type UserRanking = auditrepo.UserRanking
type ActionGroupItem = auditrepo.ActionGroupItem
type ResourceTypeGroupItem = auditrepo.ResourceTypeGroupItem
type TrendItem = auditrepo.TrendItem
type HighRiskLog = auditrepo.HighRiskLog
type RiskRule = auditrepo.RiskRule
type HighRiskRule = auditrepo.HighRiskRule

var RiskRules = auditrepo.RiskRules
var HighRiskRules = auditrepo.HighRiskRules

func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return auditrepo.NewAuditLogRepository(db)
}

func IsHighRisk(action, resourceType string) bool {
	return auditrepo.IsHighRisk(action, resourceType)
}

func GetRiskLevel(action, resourceType string) string {
	return auditrepo.GetRiskLevel(action, resourceType)
}

func GetRiskReason(action, resourceType string) string {
	return auditrepo.GetRiskReason(action, resourceType)
}

func buildHighRiskCondition() string {
	return auditrepo.BuildHighRiskCondition()
}
