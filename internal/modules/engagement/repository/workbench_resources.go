package repository

import (
	"context"
	"strings"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ResourceCount 通用资源计数
type ResourceCount struct {
	Total       int64   `json:"total"`
	Enabled     *int64  `json:"enabled,omitempty"`
	Offline     *int64  `json:"offline,omitempty"`
	NeedsReview *int64  `json:"needs_review,omitempty"`
	Channels    *int64  `json:"channels,omitempty"`
	Types       *string `json:"types,omitempty"`
	Admins      *int64  `json:"admins,omitempty"`
}

// ResourceOverview 资源概览
type ResourceOverview struct {
	Flows                 ResourceCount `json:"flows"`
	Rules                 ResourceCount `json:"rules"`
	Hosts                 ResourceCount `json:"hosts"`
	Playbooks             ResourceCount `json:"playbooks"`
	Schedules             ResourceCount `json:"schedules"`
	NotificationTemplates ResourceCount `json:"notification_templates"`
	Secrets               ResourceCount `json:"secrets"`
	Users                 ResourceCount `json:"users"`
}

// GetResourceOverview 获取各模块资源总数（按权限过滤子模块）
func (r *WorkbenchRepository) GetResourceOverview(ctx context.Context, permissions []string) (*ResourceOverview, error) {
	overview := &ResourceOverview{}
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }

	if repoHasPermission(permissions, "healing:flows:view") {
		if err := workbenchFillEnabledCount(newDB(), &model.HealingFlow{}, "is_active = ?", &overview.Flows, true); err != nil {
			return nil, err
		}
	}
	if repoHasPermission(permissions, "healing:rules:view") {
		if err := workbenchFillEnabledCount(newDB(), &model.HealingRule{}, "is_active = ?", &overview.Rules, true); err != nil {
			return nil, err
		}
	}
	if repoHasPermission(permissions, "plugin:list") {
		if err := workbenchFillOfflineCount(newDB(), &model.CMDBItem{}, "status != ?", &overview.Hosts, "active"); err != nil {
			return nil, err
		}
		if err := workbenchFillSecretsOverview(newDB(), &overview.Secrets); err != nil {
			return nil, err
		}
	}
	if repoHasPermission(permissions, "playbook:list") {
		if err := workbenchFillNeedsReviewCount(newDB(), &model.Playbook{}, "status = ?", &overview.Playbooks, "draft"); err != nil {
			return nil, err
		}
	}
	if repoHasPermission(permissions, "task:list") {
		if err := workbenchFillEnabledCount(newDB(), &model.ExecutionSchedule{}, "enabled = ?", &overview.Schedules, true); err != nil {
			return nil, err
		}
	}
	if repoHasPermission(permissions, "template:list") {
		if err := workbenchFillTemplateOverview(newDB(), &overview.NotificationTemplates); err != nil {
			return nil, err
		}
	}
	if repoHasPermission(permissions, "user:list") {
		if err := r.workbenchFillUserOverview(ctx, &overview.Users); err != nil {
			return nil, err
		}
	}

	return overview, nil
}

func workbenchFillEnabledCount(db *gorm.DB, entity any, condition string, dest *ResourceCount, args ...any) error {
	total, err := workbenchCountWhere(db, entity, "")
	if err != nil {
		return err
	}
	enabled, err := workbenchCountWhere(db, entity, condition, args...)
	if err != nil {
		return err
	}

	dest.Total = total
	dest.Enabled = &enabled
	return nil
}

func workbenchFillOfflineCount(db *gorm.DB, entity any, condition string, dest *ResourceCount, args ...any) error {
	total, err := workbenchCountWhere(db, entity, "")
	if err != nil {
		return err
	}
	offline, err := workbenchCountWhere(db, entity, condition, args...)
	if err != nil {
		return err
	}

	dest.Total = total
	dest.Offline = &offline
	return nil
}

func workbenchFillNeedsReviewCount(db *gorm.DB, entity any, condition string, dest *ResourceCount, args ...any) error {
	total, err := workbenchCountWhere(db, entity, "")
	if err != nil {
		return err
	}
	needsReview, err := workbenchCountWhere(db, entity, condition, args...)
	if err != nil {
		return err
	}

	dest.Total = total
	dest.NeedsReview = &needsReview
	return nil
}

func workbenchFillSecretsOverview(db *gorm.DB, dest *ResourceCount) error {
	total, err := workbenchCountWhere(db, &model.SecretsSource{}, "")
	if err != nil {
		return err
	}

	var secretTypes []string
	if err := db.Session(&gorm.Session{}).Model(&model.SecretsSource{}).Distinct("type").Pluck("type", &secretTypes).Error; err != nil {
		return err
	}

	dest.Total = total
	types := joinTypes(secretTypes)
	dest.Types = &types
	return nil
}

func workbenchFillTemplateOverview(db *gorm.DB, dest *ResourceCount) error {
	total, err := workbenchCountWhere(db, &model.NotificationTemplate{}, "")
	if err != nil {
		return err
	}
	channels, err := workbenchCountWhere(db, &model.NotificationChannel{}, "")
	if err != nil {
		return err
	}

	dest.Total = total
	dest.Channels = &channels
	return nil
}

func (r *WorkbenchRepository) workbenchFillUserOverview(ctx context.Context, dest *ResourceCount) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	total, err := workbenchCountDistinctTenantUsers(r.db.WithContext(ctx), tenantID)
	if err != nil {
		return err
	}
	admins, err := workbenchCountTenantAdmins(r.db.WithContext(ctx), tenantID)
	if err != nil {
		return err
	}

	dest.Total = total
	dest.Admins = &admins
	return nil
}

func workbenchCountDistinctTenantUsers(db *gorm.DB, tenantID uuid.UUID) (int64, error) {
	var count int64
	err := db.Model(&model.UserTenantRole{}).
		Where("tenant_id = ?", tenantID).
		Distinct("user_id").
		Count(&count).Error
	return count, err
}

func workbenchCountTenantAdmins(db *gorm.DB, tenantID uuid.UUID) (int64, error) {
	var count int64
	err := db.Table("user_tenant_roles AS utr").
		Joins("JOIN roles r ON utr.role_id = r.id").
		Where("utr.tenant_id = ?", tenantID).
		Where("r.name IN ?", []string{"admin", "super_admin"}).
		Distinct("utr.user_id").
		Count(&count).Error
	return count, err
}

// repoHasPermission 检查用户是否有指定权限（含通配符匹配）
func repoHasPermission(userPermissions []string, required string) bool {
	for _, permission := range userPermissions {
		if permission == "*" || permission == required {
			return true
		}
		if strings.HasSuffix(permission, ":*") {
			module := strings.TrimSuffix(permission, ":*")
			if strings.HasPrefix(required, module+":") {
				return true
			}
		}
	}
	return false
}

// joinTypes 拼接类型字符串
func joinTypes(types []string) string {
	if len(types) == 0 {
		return ""
	}

	parts := make([]string, 0, len(types))
	for _, value := range types {
		switch value {
		case "ssh":
			parts = append(parts, "SSH")
		case "api":
			parts = append(parts, "API")
		default:
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " + ")
}
