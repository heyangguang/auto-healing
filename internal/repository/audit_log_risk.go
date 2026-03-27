package repository

import auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"

const (
	RiskLevelLow      = auditrepo.RiskLevelLow
	RiskLevelMedium   = auditrepo.RiskLevelMedium
	RiskLevelHigh     = auditrepo.RiskLevelHigh
	RiskLevelCritical = auditrepo.RiskLevelCritical
)

type RiskRule = auditrepo.RiskRule
type HighRiskRule = auditrepo.HighRiskRule

var RiskRules = auditrepo.RiskRules
var HighRiskRules = auditrepo.HighRiskRules

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
