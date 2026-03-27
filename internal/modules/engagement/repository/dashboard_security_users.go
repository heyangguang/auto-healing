package repository

import (
	"context"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SecretsSection struct {
	Total      int64         `json:"total"`
	Active     int64         `json:"active"`
	ByType     []StatusCount `json:"by_type"`
	ByAuthType []StatusCount `json:"by_auth_type"`
}

func (r *DashboardRepository) GetSecretsSection(ctx context.Context) (*SecretsSection, error) {
	section := &SecretsSection{}
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }
	if err := countModel(newDB(), &projection.SecretsSource{}, &section.Total); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "active"), &projection.SecretsSource{}, &section.Active); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.SecretsSource{}, "type", &section.ByType); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.SecretsSource{}, "auth_type", &section.ByAuthType); err != nil {
		return nil, err
	}
	return section, nil
}

type UsersSection struct {
	Total        int64       `json:"total"`
	Active       int64       `json:"active"`
	RolesTotal   int64       `json:"roles_total"`
	RecentLogins []LoginItem `json:"recent_logins"`
}

type LoginItem struct {
	ID          uuid.UUID  `json:"id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	LastLoginAt *time.Time `json:"last_login_at"`
	LastLoginIP string     `json:"last_login_ip"`
}

func (r *DashboardRepository) GetUsersSection(ctx context.Context) (*UsersSection, error) {
	section := &UsersSection{}
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	db := r.db.WithContext(ctx)

	if err := countModel(db.Table("users").
		Joins("JOIN user_tenant_roles utr ON utr.user_id = users.id").
		Where("utr.tenant_id = ?", tenantID).
		Distinct("users.id"), &model.User{}, &section.Total); err != nil {
		return nil, err
	}
	if err := countModel(db.Table("users").
		Joins("JOIN user_tenant_roles utr ON utr.user_id = users.id").
		Where("utr.tenant_id = ? AND users.status = ?", tenantID, "active").
		Distinct("users.id"), &model.User{}, &section.Active); err != nil {
		return nil, err
	}
	if err := countModel(db.Model(&model.Role{}).
		Where("scope = ?", "tenant").
		Where("tenant_id IS NULL OR tenant_id = ?", tenantID), &model.Role{}, &section.RolesTotal); err != nil {
		return nil, err
	}
	recentLogins, err := listRecentLogins(db.Table("users").
		Joins("JOIN user_tenant_roles utr ON utr.user_id = users.id").
		Where("utr.tenant_id = ? AND users.last_login_at IS NOT NULL", tenantID).
		Distinct("users.id").
		Order("users.last_login_at DESC").
		Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentLogins = recentLogins
	return section, nil
}

func listRecentLogins(query *gorm.DB) ([]LoginItem, error) {
	var users []model.User
	if err := query.Find(&users).Error; err != nil {
		return nil, err
	}
	items := make([]LoginItem, 0, len(users))
	for _, user := range users {
		items = append(items, LoginItem{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			LastLoginAt: user.LastLoginAt,
			LastLoginIP: user.LastLoginIP,
		})
	}
	return items, nil
}
