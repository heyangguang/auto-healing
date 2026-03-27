package audit

import (
	"strings"
	"testing"
)

func TestGetRiskLevelNormalizesTenantPrefixedResources(t *testing.T) {
	tests := []struct {
		action       string
		resourceType string
		want         string
	}{
		{"reset_password", "tenant-users", RiskLevelHigh},
		{"assign_role", "tenant-users", RiskLevelCritical},
		{"assign_permission", "tenant-roles", RiskLevelCritical},
		{"deactivate", "tenant-plugins", RiskLevelHigh},
		{"execute", "tenant-execution-tasks", RiskLevelMedium},
		{"trigger", "tenant-incidents", RiskLevelMedium},
		{"approve", "tenant-healing", RiskLevelMedium},
	}

	for _, tc := range tests {
		if got := GetRiskLevel(tc.action, tc.resourceType); got != tc.want {
			t.Fatalf("GetRiskLevel(%q, %q) = %q, want %q", tc.action, tc.resourceType, got, tc.want)
		}
		if GetRiskReason(tc.action, tc.resourceType) == "" {
			t.Fatalf("GetRiskReason(%q, %q) should not be empty", tc.action, tc.resourceType)
		}
	}
}

func TestBuildHighRiskConditionIncludesTenantVariants(t *testing.T) {
	sql := BuildHighRiskCondition()
	for _, want := range []string{
		"tenant-users",
		"tenant-roles",
		"tenant-plugins",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("BuildHighRiskCondition() missing %q in %q", want, sql)
		}
	}
}
