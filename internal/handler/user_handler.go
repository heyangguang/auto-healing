package handler

import (
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserHandler 用户管理处理器
type UserHandler struct {
	userRepo *repository.UserRepository
	roleRepo *repository.RoleRepository
	authSvc  *authService.Service
}

// NewUserHandler 创建用户处理器
func NewUserHandler(authSvc *authService.Service) *UserHandler {
	return &UserHandler{
		userRepo: repository.NewUserRepository(),
		roleRepo: repository.NewRoleRepository(),
		authSvc:  authSvc,
	}
}

// RoleHandler 角色管理处理器
type RoleHandler struct {
	roleRepo *repository.RoleRepository
	permRepo *repository.PermissionRepository
}

// NewRoleHandler 创建角色处理器
func NewRoleHandler() *RoleHandler {
	return &RoleHandler{
		roleRepo: repository.NewRoleRepository(),
		permRepo: repository.NewPermissionRepository(),
	}
}

// PermissionHandler 权限处理器
type PermissionHandler struct {
	permRepo *repository.PermissionRepository
}

// NewPermissionHandler 创建权限处理器
func NewPermissionHandler() *PermissionHandler {
	return &PermissionHandler{
		permRepo: repository.NewPermissionRepository(),
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
