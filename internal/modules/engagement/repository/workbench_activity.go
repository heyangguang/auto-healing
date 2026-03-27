package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"github.com/google/uuid"
)

var activityActionText = map[string]string{
	"create":     "创建",
	"update":     "更新",
	"delete":     "删除",
	"execute":    "执行",
	"enable":     "启用",
	"disable":    "禁用",
	"activate":   "激活",
	"deactivate": "停用",
	"approve":    "审批通过",
	"reject":     "审批拒绝",
	"dismiss":    "忽略",
	"login":      "登录",
	"logout":     "退出",
}

var activityTypeText = map[string]string{
	"execution_task":         "执行任务",
	"execution_run":          "执行运行",
	"execution-tasks":        "执行任务",
	"execution-runs":         "执行运行",
	"git-repos":              "Git仓库",
	"playbooks":              "Playbook",
	"execution-schedules":    "定时任务",
	"healing_flow":           "自愈流程",
	"healing_rule":           "自愈规则",
	"healing_instance":       "自愈实例",
	"healing-flows":          "自愈流程",
	"healing-rules":          "自愈规则",
	"healing-instances":      "自愈实例",
	"healing-approvals":      "审批任务",
	"cmdb_item":              "资产",
	"tenant-cmdb":            "资产",
	"plugin":                 "插件",
	"tenant-plugins":         "插件",
	"tenant-incidents":       "事件",
	"tenant-secrets-sources": "密钥源",
	"playbook":               "Playbook",
	"schedule":               "定时任务",
	"notification":           "通知",
	"secrets":                "密钥",
	"user":                   "用户",
	"tenant-users":           "用户",
	"role":                   "角色",
	"tenant-roles":           "角色",
	"tenant-permissions":     "权限",
	"site_message":           "站内信",
	"tenant-site-messages":   "站内信",
	"tenant":                 "租户",
}

// ActivityItem 活动动态项
type ActivityItem struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// GetRecentActivities 从审计日志获取最近的活动动态
func (r *WorkbenchRepository) GetRecentActivities(ctx context.Context, limit int) ([]ActivityItem, error) {
	if limit <= 0 {
		limit = 10
	}

	var logs []projection.AuditLog
	if err := r.tenantDB(ctx).Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}

	items := make([]ActivityItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, ActivityItem{
			ID:        log.ID,
			Type:      mapResourceTypeToActivityType(log.ResourceType),
			Text:      buildActivityText(log.Action, log.ResourceType, log.ResourceName),
			CreatedAt: log.CreatedAt,
		})
	}
	return items, nil
}

func mapResourceTypeToActivityType(resourceType string) string {
	switch resourceType {
	case "execution_task", "execution_run", "execution-tasks", "execution-runs", "git-repos", "playbooks", "execution-schedules":
		return "execution"
	case "healing_flow", "healing-flow", "healing-flows", "healing-instance", "healing-instances", "healing-approval", "healing-approvals":
		return "flow"
	case "healing_rule", "healing-rule", "healing-rules":
		return "rule"
	case "tenant-users", "tenant-roles", "tenant-permissions", "users", "roles":
		return "access"
	case "tenant-plugins", "tenant-incidents", "tenant-cmdb", "tenant-secrets-sources", "tenant-site-messages":
		return "ops"
	case "auth":
		return "system"
	default:
		if strings.HasPrefix(resourceType, "healing-") {
			return "flow"
		}
		if strings.HasPrefix(resourceType, "tenant-") {
			return "ops"
		}
		return "system"
	}
}

func buildActivityText(action, resourceType, resourceName string) string {
	if text, ok := authActivityText(action, resourceType); ok {
		return text
	}
	actionText := activityLabel(activityActionText, action)
	typeText := activityLabel(activityTypeText, resourceType)
	if resourceName != "" {
		return fmt.Sprintf("%s%s：%s", actionText, typeText, resourceName)
	}
	return fmt.Sprintf("%s了%s", actionText, typeText)
}

func authActivityText(action, resourceType string) (string, bool) {
	if resourceType != "auth" {
		return "", false
	}
	switch action {
	case "login":
		return "用户登录系统", true
	case "logout":
		return "用户退出系统", true
	default:
		return "", false
	}
}

func activityLabel(values map[string]string, key string) string {
	if value, ok := values[key]; ok && value != "" {
		return value
	}
	return key
}
