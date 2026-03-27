package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserListParams 用户列表查询参数
type UserListParams struct {
	Page         int
	PageSize     int
	Status       string
	Username     query.StringFilter
	Email        query.StringFilter
	DisplayName  query.StringFilter
	RoleID       string
	CreatedFrom  string
	CreatedTo    string
	SortField    string
	SortOrder    string
	PlatformOnly bool
}

// SimpleUser 简要用户信息（用于下拉选择等场景）
type SimpleUser struct {
	ID              uuid.UUID `json:"id"`
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	Status          string    `json:"status"`
	IsPlatformAdmin bool      `json:"is_platform_admin"`
}

// List 获取用户列表（支持按字段搜索、组合搜索、排序）
func (r *UserRepository) List(ctx context.Context, params *UserListParams) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	queryBuilder := buildUserListQuery(r.db.WithContext(ctx), params)
	if err := queryBuilder.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (params.Page - 1) * params.PageSize
	err := queryBuilder.Preload("Roles").
		Offset(offset).
		Limit(params.PageSize).
		Order(userListOrderClause(params)).
		Find(&users).Error
	return users, total, err
}

func buildUserListQuery(q *gorm.DB, params *UserListParams) *gorm.DB {
	if params.Status != "" {
		q = q.Where("status = ?", params.Status)
	}
	q = query.ApplyStringFilter(q, "username", params.Username)
	q = query.ApplyStringFilter(q, "email", params.Email)
	q = query.ApplyStringFilter(q, "display_name", params.DisplayName)
	if params.PlatformOnly {
		q = q.Where("is_platform_admin = true")
	} else if params.RoleID != "" {
		q = q.Where("id IN (SELECT user_id FROM user_platform_roles WHERE role_id = ?)", params.RoleID)
	}
	if params.CreatedFrom != "" {
		q = q.Where("created_at >= ?", params.CreatedFrom)
	}
	if params.CreatedTo != "" {
		q = q.Where("created_at <= ?", params.CreatedTo)
	}
	return q.Model(&model.User{})
}

func userListOrderClause(params *UserListParams) string {
	const defaultOrder = "created_at DESC"
	allowedSortFields := map[string]bool{
		"username":      true,
		"email":         true,
		"display_name":  true,
		"created_at":    true,
		"last_login_at": true,
		"status":        true,
	}
	if params.SortField == "" || !allowedSortFields[params.SortField] {
		return defaultOrder
	}
	if params.SortOrder == "asc" {
		return params.SortField + " ASC"
	}
	return params.SortField + " DESC"
}

// ListSimple 获取简要用户列表（轻量接口，不加载关联）
func (r *UserRepository) ListSimple(ctx context.Context, name string, status string) ([]SimpleUser, error) {
	var users []SimpleUser
	queryBuilder := r.db.WithContext(ctx).
		Model(&model.User{}).
		Select("id, username, display_name, status, is_platform_admin")
	if status != "" {
		queryBuilder = queryBuilder.Where("status = ?", status)
	}
	if name != "" {
		like := "%" + name + "%"
		queryBuilder = queryBuilder.Where("username ILIKE ? OR display_name ILIKE ?", like, like)
	}
	err := queryBuilder.Order("username ASC").Limit(500).Find(&users).Error
	return users, err
}
