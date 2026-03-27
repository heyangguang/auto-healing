package handler

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetCurrentUser 获取当前用户信息
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, ok := authCurrentUserID(c)
	if !ok {
		return
	}

	userInfo, err := h.authSvc.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			response.NotFound(c, "用户不存在")
			return
		}
		respondInternalError(c, "AUTH", "获取当前用户信息失败", err)
		return
	}
	if err := h.applyEffectivePermissions(c, userID, userInfo); err != nil {
		respondInternalError(c, "AUTH", "获取当前用户信息失败", err)
		return
	}
	response.Success(c, userInfo)
}

func authCurrentUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return uuid.Nil, false
	}
	return userID, true
}

func (h *AuthHandler) applyEffectivePermissions(c *gin.Context, userID uuid.UUID, userInfo *authService.UserInfo) error {
	if middleware.IsImpersonating(c) {
		perms := middleware.GetPermissions(c)
		if perms == nil {
			return fmt.Errorf("impersonation 权限上下文缺失")
		}
		userInfo.Permissions = append([]string(nil), perms...)
		return nil
	}
	if userInfo.IsPlatformAdmin {
		return nil
	}

	tenantIDStr, exists := c.Get(middleware.TenantIDKey)
	if !exists || tenantIDStr == nil {
		return nil
	}
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return fmt.Errorf("解析当前租户失败: %w", err)
	}
	tenantPerms, err := repository.NewPermissionRepository().GetTenantPermissionCodes(c.Request.Context(), userID, tenantID)
	if err != nil {
		return err
	}
	userInfo.Permissions = tenantPerms
	tenantRoles, err := repository.NewTenantRepository().GetUserTenantRoles(c.Request.Context(), userID, tenantID)
	if err != nil {
		return err
	}
	roleNames := make([]string, len(tenantRoles))
	for i, role := range tenantRoles {
		roleNames[i] = role.Name
	}
	userInfo.Roles = roleNames
	return nil
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, ok := authCurrentUserID(c)
	if !ok {
		return
	}

	var req authService.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.authSvc.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		respondChangePasswordError(c, err)
		return
	}
	response.Message(c, "密码修改成功")
}

// GetProfile 获取当前用户详细信息（个人中心使用）
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, ok := authCurrentUserID(c)
	if !ok {
		return
	}

	profile, err := h.authSvc.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			response.NotFound(c, "用户不存在")
			return
		}
		respondInternalError(c, "AUTH", "获取个人资料失败", err)
		return
	}
	response.Success(c, profile)
}

// UpdateProfile 更新个人信息
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, ok := authCurrentUserID(c)
	if !ok {
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.authSvc.UpdateProfile(c.Request.Context(), userID, req.DisplayName, req.Email, req.Phone); err != nil {
		respondUpdateProfileError(c, err)
		return
	}
	response.Message(c, "更新成功")
}

// GetLoginHistory 获取当前用户的登录历史
func (h *AuthHandler) GetLoginHistory(c *gin.Context) {
	userID, ok := authCurrentUserID(c)
	if !ok {
		return
	}

	items, err := h.loadLoginHistoryItems(c, userID, authHistoryLimit(c, 10))
	if err != nil {
		respondProfileAuditQueryError(c, "获取登录历史失败", err)
		return
	}
	response.Success(c, map[string]interface{}{"items": items})
}

func authHistoryLimit(c *gin.Context, defaultValue int) int {
	limit := defaultValue
	if raw := c.Query("limit"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			limit = value
		}
	}
	return limit
}

func (h *AuthHandler) loadLoginHistoryItems(c *gin.Context, userID uuid.UUID, limit int) ([]LoginHistoryItem, error) {
	if middleware.IsPlatformAdmin(c) {
		logs, err := h.platformAuditRepo.GetUserLoginHistory(c.Request.Context(), userID, limit)
		if err != nil {
			return nil, err
		}
		return buildPlatformLoginHistoryItems(logs), nil
	}

	tenantID, err := authTenantIDOrError(c)
	if err != nil {
		return nil, err
	}
	logs, err := h.auditRepo.GetUserLoginHistory(c.Request.Context(), userID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	return buildTenantLoginHistoryItems(logs), nil
}

// GetProfileActivities 获取当前用户的操作记录（排除 login/logout）
func (h *AuthHandler) GetProfileActivities(c *gin.Context) {
	userID, ok := authCurrentUserID(c)
	if !ok {
		return
	}

	items, err := h.loadProfileActivityItems(c, userID, authHistoryLimit(c, 15))
	if err != nil {
		respondProfileAuditQueryError(c, "获取操作记录失败", err)
		return
	}
	response.Success(c, map[string]interface{}{"items": items})
}

func (h *AuthHandler) loadProfileActivityItems(c *gin.Context, userID uuid.UUID, limit int) ([]ProfileActivityItem, error) {
	if middleware.IsPlatformAdmin(c) {
		logs, err := h.platformAuditRepo.GetUserActivities(c.Request.Context(), userID, limit)
		if err != nil {
			return nil, err
		}
		return buildPlatformActivityItems(logs), nil
	}

	tenantID, err := authTenantIDOrError(c)
	if err != nil {
		return nil, err
	}
	logs, err := h.auditRepo.GetUserActivities(c.Request.Context(), userID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	return buildTenantActivityItems(logs), nil
}

func buildPlatformLoginHistoryItems(logs []model.PlatformAuditLog) []LoginHistoryItem {
	items := make([]LoginHistoryItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, LoginHistoryItem{
			ID:           log.ID,
			Action:       log.Action,
			IPAddress:    log.IPAddress,
			UserAgent:    log.UserAgent,
			Status:       log.Status,
			ErrorMessage: sanitizeLoginHistoryErrorMessage(log.Status, log.ResponseStatus, log.ErrorMessage),
			CreatedAt:    log.CreatedAt,
		})
	}
	if items == nil {
		return []LoginHistoryItem{}
	}
	return items
}

func buildTenantLoginHistoryItems(logs []model.AuditLog) []LoginHistoryItem {
	items := make([]LoginHistoryItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, LoginHistoryItem{
			ID:           log.ID,
			Action:       log.Action,
			IPAddress:    log.IPAddress,
			UserAgent:    log.UserAgent,
			Status:       log.Status,
			ErrorMessage: sanitizeLoginHistoryErrorMessage(log.Status, log.ResponseStatus, log.ErrorMessage),
			CreatedAt:    log.CreatedAt,
		})
	}
	if items == nil {
		return []LoginHistoryItem{}
	}
	return items
}

func buildPlatformActivityItems(logs []model.PlatformAuditLog) []ProfileActivityItem {
	items := make([]ProfileActivityItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, ProfileActivityItem{
			ID:           log.ID,
			Action:       log.Action,
			ResourceType: log.ResourceType,
			ResourceName: log.ResourceName,
			Status:       log.Status,
			IPAddress:    log.IPAddress,
			CreatedAt:    log.CreatedAt,
		})
	}
	if items == nil {
		return []ProfileActivityItem{}
	}
	return items
}

func buildTenantActivityItems(logs []model.AuditLog) []ProfileActivityItem {
	items := make([]ProfileActivityItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, ProfileActivityItem{
			ID:           log.ID,
			Action:       log.Action,
			ResourceType: log.ResourceType,
			ResourceName: log.ResourceName,
			Status:       log.Status,
			IPAddress:    log.IPAddress,
			CreatedAt:    log.CreatedAt,
		})
	}
	if items == nil {
		return []ProfileActivityItem{}
	}
	return items
}
