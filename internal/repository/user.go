package repository

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound       = errors.New("用户不存在")
	ErrUserExists         = errors.New("用户名或邮箱已存在")
	ErrRoleNotFound       = errors.New("角色不存在")
	ErrPermissionNotFound = errors.New("权限不存在")
)

// UserRepository 用户数据仓库
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓库
func NewUserRepository() *UserRepository {
	return &UserRepository{db: database.DB}
}

// Create 创建用户
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByID 根据 ID 获取用户
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Preload("Roles.Permissions").First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

// GetByUsername 根据用户名获取用户
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Preload("Roles.Permissions").First(&user, "username = ?", username).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

// GetByEmail 根据邮箱获取用户
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Preload("Roles.Permissions").First(&user, "email = ?", email).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

// Update 更新用户
func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete 删除用户
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, "id = ?", id).Error
}

// UserListParams 用户列表查询参数
type UserListParams struct {
	Page         int
	PageSize     int
	Status       string // 按状态精确过滤
	Search       string // 全文模糊搜索（兼容旧参数）
	Username     string // 按用户名模糊搜索
	Email        string // 按邮箱模糊搜索
	DisplayName  string // 按显示名模糊搜索
	RoleID       string // 按角色 ID 精确过滤
	CreatedFrom  string // 创建时间起始（ISO 8601）
	CreatedTo    string // 创建时间截止（ISO 8601）
	SortField    string // 排序字段
	SortOrder    string // 排序方向 (asc/desc)
	PlatformOnly bool   // 仅返回有平台级角色的用户（platform_admin）
}

// List 获取用户列表（支持按字段搜索、组合搜索、排序）
func (r *UserRepository) List(ctx context.Context, params *UserListParams) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := r.db.WithContext(ctx).Model(&model.User{})

	// 按状态精确过滤
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	// 按字段独立搜索（优先于全文搜索）
	hasFieldFilter := params.Username != "" || params.Email != "" || params.DisplayName != ""
	if hasFieldFilter {
		if params.Username != "" {
			query = query.Where("username ILIKE ?", "%"+params.Username+"%")
		}
		if params.Email != "" {
			query = query.Where("email ILIKE ?", "%"+params.Email+"%")
		}
		if params.DisplayName != "" {
			query = query.Where("display_name ILIKE ?", "%"+params.DisplayName+"%")
		}
	} else if params.Search != "" {
		// 全文模糊搜索（兼容旧参数）
		like := "%" + params.Search + "%"
		query = query.Where("username ILIKE ? OR email ILIKE ? OR display_name ILIKE ?", like, like, like)
	}

	// 按角色过滤
	if params.PlatformOnly {
		// 只返回拥有平台管理员角色的用户
		query = query.Where(`id IN (
			SELECT ur.user_id FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE r.name = 'platform_admin'
		)`)
	} else if params.RoleID != "" {
		query = query.Where("id IN (SELECT user_id FROM user_roles WHERE role_id = ?)", params.RoleID)
	}

	// 按创建时间范围过滤
	if params.CreatedFrom != "" {
		query = query.Where("created_at >= ?", params.CreatedFrom)
	}
	if params.CreatedTo != "" {
		query = query.Where("created_at <= ?", params.CreatedTo)
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	orderClause := "created_at DESC" // 默认排序
	allowedSortFields := map[string]bool{
		"username":      true,
		"email":         true,
		"display_name":  true,
		"created_at":    true,
		"last_login_at": true,
		"status":        true,
	}
	if params.SortField != "" && allowedSortFields[params.SortField] {
		direction := "DESC"
		if params.SortOrder == "asc" {
			direction = "ASC"
		}
		orderClause = params.SortField + " " + direction
	}

	offset := (params.Page - 1) * params.PageSize
	err := query.Preload("Roles").Offset(offset).Limit(params.PageSize).Order(orderClause).Find(&users).Error
	return users, total, err
}

// ExistsByUsername 检查用户名是否存在
func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// ExistsByEmail 检查邮箱是否存在
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// AssignRoles 分配角色给用户
func (r *UserRepository) AssignRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除现有角色关联
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}

		// 添加新角色关联
		for _, roleID := range roleIDs {
			userRole := model.UserRole{
				UserID: userID,
				RoleID: roleID,
			}
			if err := tx.Create(&userRole).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateLoginInfo 更新登录信息（同时解锁账户、重置失败计数）
func (r *UserRepository) UpdateLoginInfo(ctx context.Context, userID uuid.UUID, ip string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"last_login_at":      gorm.Expr("NOW()"),
			"last_login_ip":      ip,
			"failed_login_count": 0,
			"status":             "active",
			"locked_until":       nil,
		}).Error
}

// IncrementFailedLogin 增加登录失败次数，达到阈值时自动锁定账户
func (r *UserRepository) IncrementFailedLogin(ctx context.Context, userID uuid.UUID) error {
	const maxAttempts = 5
	const lockDuration = 30 * time.Minute

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 增加失败计数
		if err := tx.Model(&model.User{}).Where("id = ?", userID).
			Update("failed_login_count", gorm.Expr("failed_login_count + 1")).Error; err != nil {
			return err
		}
		// 检查是否需要锁定
		var count int
		if err := tx.Model(&model.User{}).Where("id = ?", userID).
			Select("failed_login_count").Scan(&count).Error; err != nil {
			return err
		}
		if count >= maxAttempts {
			lockUntil := time.Now().Add(lockDuration)
			return tx.Model(&model.User{}).Where("id = ?", userID).
				Updates(map[string]interface{}{
					"status":       "locked",
					"locked_until": lockUntil,
				}).Error
		}
		return nil
	})
}

// UpdatePassword 更新密码
func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"password_hash":       passwordHash,
			"password_changed_at": gorm.Expr("NOW()"),
		}).Error
}

// SimpleUser 简要用户信息（用于下拉选择等场景）
type SimpleUser struct {
	ID              uuid.UUID `json:"id"`
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	Status          string    `json:"status"`
	IsPlatformAdmin bool      `json:"is_platform_admin"`
}

// ListSimple 获取简要用户列表（轻量接口，不加载关联）
func (r *UserRepository) ListSimple(ctx context.Context, search string, status string) ([]SimpleUser, error) {
	var users []SimpleUser

	query := r.db.WithContext(ctx).
		Model(&model.User{}).
		Select(`id, username, display_name, status,
			EXISTS(SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
				WHERE ur.user_id = users.id AND r.name = 'platform_admin'
			) AS is_platform_admin`)

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("username ILIKE ? OR display_name ILIKE ?", like, like)
	}

	err := query.Order("username ASC").Limit(500).Find(&users).Error
	return users, err
}
