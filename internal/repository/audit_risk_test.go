package repository

import (
	"strings"
	"testing"

	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
)

func TestGetRiskLevelNormalizesTenantPrefixedResources(t *testing.T) {
	tests := []struct {
		action       string
		resourceType string
		want         string
	}{
		{"reset_password", "tenant-users", auditrepo.RiskLevelHigh},
		{"assign_role", "tenant-users", auditrepo.RiskLevelCritical},
		{"assign_permission", "tenant-roles", auditrepo.RiskLevelCritical},
		{"deactivate", "tenant-plugins", auditrepo.RiskLevelHigh},
		{"execute", "tenant-execution-tasks", auditrepo.RiskLevelMedium},
		{"trigger", "tenant-incidents", auditrepo.RiskLevelMedium},
		{"approve", "tenant-healing", auditrepo.RiskLevelMedium},
	}

	for _, tc := range tests {
		if got := auditrepo.GetRiskLevel(tc.action, tc.resourceType); got != tc.want {
			t.Fatalf("GetRiskLevel(%q, %q) = %q, want %q", tc.action, tc.resourceType, got, tc.want)
		}
		if auditrepo.GetRiskReason(tc.action, tc.resourceType) == "" {
			t.Fatalf("GetRiskReason(%q, %q) should not be empty", tc.action, tc.resourceType)
		}
	}
}

func TestBuildHighRiskConditionIncludesTenantVariants(t *testing.T) {
	sql := auditrepo.BuildHighRiskCondition()
	for _, want := range []string{
		"tenant-users",
		"tenant-roles",
		"tenant-plugins",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("buildHighRiskCondition() missing %q in %q", want, sql)
		}
	}
}
