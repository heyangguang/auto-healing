package httpapi

import (
	"fmt"

	"github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserHandler 用户管理处理器
type UserHandler struct {
	userRepo *accessrepo.UserRepository
	roleRepo *accessrepo.RoleRepository
	authSvc  *authService.Service
}

type UserHandlerDeps struct {
	UserRepo    *accessrepo.UserRepository
	RoleRepo    *accessrepo.RoleRepository
	AuthService *authService.Service
}

// NewUserHandler 创建用户处理器
func NewUserHandler(authSvc *authService.Service) *UserHandler {
	return NewUserHandlerWithDeps(UserHandlerDeps{
		UserRepo:    accessrepo.NewUserRepository(),
		RoleRepo:    accessrepo.NewRoleRepository(),
		AuthService: authSvc,
	})
}

func NewUserHandlerWithDeps(deps UserHandlerDeps) *UserHandler {
	return &UserHandler{
		userRepo: deps.UserRepo,
		roleRepo: deps.RoleRepo,
		authSvc:  deps.AuthService,
	}
}

// RoleHandler 角色管理处理器
type RoleHandler struct {
	roleRepo *accessrepo.RoleRepository
	permRepo *accessrepo.PermissionRepository
}

type RoleHandlerDeps struct {
	RoleRepo       *accessrepo.RoleRepository
	PermissionRepo *accessrepo.PermissionRepository
}

// NewRoleHandler 创建角色处理器
func NewRoleHandler() *RoleHandler {
	return NewRoleHandlerWithDeps(RoleHandlerDeps{
		RoleRepo:       accessrepo.NewRoleRepository(),
		PermissionRepo: accessrepo.NewPermissionRepository(),
	})
}

func NewRoleHandlerWithDeps(deps RoleHandlerDeps) *RoleHandler {
	return &RoleHandler{
		roleRepo: deps.RoleRepo,
		permRepo: deps.PermissionRepo,
	}
}

// PermissionHandler 权限处理器
type PermissionHandler struct {
	permRepo *accessrepo.PermissionRepository
}

type PermissionHandlerDeps struct {
	PermissionRepo *accessrepo.PermissionRepository
}

// NewPermissionHandler 创建权限处理器
func NewPermissionHandler() *PermissionHandler {
	return NewPermissionHandlerWithDeps(PermissionHandlerDeps{
		PermissionRepo: accessrepo.NewPermissionRepository(),
	})
}

func NewPermissionHandlerWithDeps(deps PermissionHandlerDeps) *PermissionHandler {
	return &PermissionHandler{
		permRepo: deps.PermissionRepo,
	}
}

// RoleWithStats 角色+统计
type RoleWithStats struct {
	model.Role
	UserCount       int64 `json:"user_count"`
	PermissionCount int64 `json:"permission_count"`
}

// 请求结构体
type UpdateUserRequest struct {
	DisplayName string     `json:"display_name"`
	Phone       string     `json:"phone"`
	Status      string     `json:"status"`
	RoleID      *uuid.UUID `json:"role_id"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type AssignRolesRequest struct {
	RoleIDs []uuid.UUID `json:"role_ids" binding:"required"`
}

type UpdateRoleRequest struct {
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

type AssignPermissionsRequest struct {
	PermissionIDs []uuid.UUID `json:"permission_ids" binding:"required"`
}

// 辅助函数
func getQueryInt(c *gin.Context, key string, defaultValue int) int {
	if val := c.Query(key); val != "" {
		var result int
		if _, err := fmt.Sscanf(val, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}
