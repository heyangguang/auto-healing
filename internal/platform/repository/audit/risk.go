package audit

import (
	"fmt"
	"strings"
)

const (
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

type RiskRule struct {
	Action       string
	ResourceType string
	Level        string
	Reason       string
}

var RiskRules = []RiskRule{
	{"impersonation_enter", "impersonation", RiskLevelCritical, "提权进入租户"},
	{"impersonation_exit", "impersonation", RiskLevelCritical, "提权退出租户"},
	{"assign_role", "users", RiskLevelCritical, "变更用户角色"},
	{"assign_permission", "roles", RiskLevelCritical, "变更角色权限"},
	{"delete", "*", RiskLevelHigh, "删除操作"},
	{"reset_password", "users", RiskLevelHigh, "管理员重置用户密码"},
	{"disable", "*", RiskLevelHigh, "禁用资源"},
	{"deactivate", "plugins", RiskLevelHigh, "停用插件"},
	{"cancel", "*", RiskLevelHigh, "取消执行中的任务"},
	{"execute", "execution-tasks", RiskLevelMedium, "执行指令/Playbook"},
	{"trigger", "incidents", RiskLevelMedium, "手动触发自愈流程"},
	{"dismiss", "incidents", RiskLevelMedium, "忽略待触发工单"},
	{"approve", "healing", RiskLevelMedium, "审批通过自愈流程"},
	{"reject", "healing", RiskLevelMedium, "审批拒绝自愈流程"},
	{"dry_run", "healing", RiskLevelMedium, "自愈流程试运行"},
	{"create", "*", RiskLevelMedium, "创建资源"},
	{"update", "*", RiskLevelMedium, "更新资源"},
	{"sync", "*", RiskLevelMedium, "同步操作"},
}

type HighRiskRule = RiskRule

var HighRiskRules = RiskRules

func IsHighRisk(action, resourceType string) bool {
	level := GetRiskLevel(action, resourceType)
	return level == RiskLevelHigh || level == RiskLevelCritical
}

func GetRiskLevel(action, resourceType string) string {
	normalized := normalizeRiskResourceType(resourceType)
	for _, rule := range RiskRules {
		if (rule.Action == "*" || rule.Action == action) &&
			(rule.ResourceType == "*" || rule.ResourceType == resourceType || rule.ResourceType == normalized) {
			return rule.Level
		}
	}
	return RiskLevelLow
}

func GetRiskReason(action, resourceType string) string {
	normalized := normalizeRiskResourceType(resourceType)
	for _, rule := range RiskRules {
		if (rule.Action == "*" || rule.Action == action) &&
			(rule.ResourceType == "*" || rule.ResourceType == resourceType || rule.ResourceType == normalized) {
			return rule.Reason
		}
	}
	return ""
}

func BuildHighRiskCondition() string {
	return buildHighRiskCondition()
}

func buildHighRiskCondition() string {
	conditions := make([]string, 0, len(RiskRules))
	for _, rule := range RiskRules {
		if rule.Level != RiskLevelHigh && rule.Level != RiskLevelCritical {
			continue
		}
		conditions = append(conditions, highRiskSQL(rule)...)
	}
	if len(conditions) == 0 {
		return "1=0"
	}
	return strings.Join(conditions, " OR ")
}

func highRiskSQL(rule RiskRule) []string {
	switch {
	case rule.Action == "*" && rule.ResourceType == "*":
		return []string{"1=1"}
	case rule.Action == "*":
		return []string{riskResourceCondition("resource_type", riskResourceVariants(rule.ResourceType))}
	case rule.ResourceType == "*":
		return []string{fmt.Sprintf("action = '%s'", rule.Action)}
	default:
		return []string{fmt.Sprintf("(action = '%s' AND %s)", rule.Action, riskResourceCondition("resource_type", riskResourceVariants(rule.ResourceType)))}
	}
}

func riskResourceCondition(column string, variants []string) string {
	if len(variants) == 1 {
		return fmt.Sprintf("%s = '%s'", column, variants[0])
	}
	return fmt.Sprintf("%s IN (%s)", column, quoteSQLStrings(variants))
}

func normalizeRiskResourceType(resourceType string) string {
	for _, prefix := range []string{"tenant-", "common-", "platform-"} {
		if strings.HasPrefix(resourceType, prefix) {
			return strings.TrimPrefix(resourceType, prefix)
		}
	}
	return resourceType
}

func riskResourceVariants(resourceType string) []string {
	seen := map[string]bool{}
	var variants []string
	add := func(value string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		variants = append(variants, value)
	}

	add(resourceType)
	add("tenant-" + resourceType)
	add("common-" + resourceType)
	add("platform-" + resourceType)
	return variants
}

func quoteSQLStrings(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("'%s'", value))
	}
	return strings.Join(quoted, ", ")
}
