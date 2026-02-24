package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authSvc           *authService.Service
	jwtSvc            *jwt.Service
	auditRepo         *repository.AuditLogRepository
	platformAuditRepo *repository.PlatformAuditLogRepository
	userRepo          *repository.UserRepository
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	blacklistStore := database.NewTokenBlacklistStore()
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          cfg.JWT.Secret,
		AccessTokenTTL:  cfg.JWT.AccessTokenTTL(),
		RefreshTokenTTL: cfg.JWT.RefreshTokenTTL(),
		Issuer:          cfg.JWT.Issuer,
	}, blacklistStore)

	return &AuthHandler{
		authSvc:           authService.NewService(jwtSvc),
		jwtSvc:            jwtSvc,
		auditRepo:         repository.NewAuditLogRepository(),
		platformAuditRepo: repository.NewPlatformAuditLogRepository(),
		userRepo:          repository.NewUserRepository(),
	}
}

// GetJWTService 获取 JWT 服务
func (h *AuthHandler) GetJWTService() *jwt.Service {
	return h.jwtSvc
}

// GetAuthService 获取认证服务（供邀请注册等公开接口使用）
func (h *AuthHandler) GetAuthService() *authService.Service {
	return h.authSvc
}

// Login 用户登录（保持原有响应格式以兼容前端）
func (h *AuthHandler) Login(c *gin.Context) {
	var req authService.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	clientIP := middleware.NormalizeIP(c.ClientIP())
	userAgent := c.Request.UserAgent()
	startTime := time.Now()

	resp, err := h.authSvc.Login(c.Request.Context(), &req, clientIP)
	if err != nil {
		// 登录失败 — 记录审计日志（通过用户名查找用户判断平台/租户）
		go h.writeLoginAuditLog(nil, req.Username, clientIP, userAgent, "failed", err.Error(), startTime, false, "")
		response.Unauthorized(c, ToBusinessError(err))
		return
	}

	// 登录成功 — 记录审计日志
	userID := resp.User.ID
	go h.writeLoginAuditLog(&userID, resp.User.Username, clientIP, userAgent, "success", "", startTime, resp.User.IsPlatformAdmin, resp.CurrentTenantID)

	// 登录响应保持原格式（包含 token 字段）
	c.JSON(http.StatusOK, resp)
}

// writeLoginAuditLog 异步写入登录审计日志
// isPlatformAdmin: 是否是平台管理员（决定写入哪张表）
// defaultTenantID: 用户的默认租户 ID（租户用户登录时设置，确保登录日志有 tenant_id）
func (h *AuthHandler) writeLoginAuditLog(userID *uuid.UUID, username, ipAddress, userAgent, status, errorMsg string, createdAt time.Time, isPlatformAdmin bool, defaultTenantID string) {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("登录审计日志记录失败 (panic)", zap.Any("error", r))
		}
	}()

	// 对于失败的登录（无 userID），尝试查找用户以判断平台/租户
	if userID == nil && username != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		user, _ := h.userRepo.GetByUsername(ctx, username)
		if user != nil {
			userID = &user.ID
			isPlatformAdmin = user.IsPlatformAdmin
			// 查找用户的默认租户
			if !isPlatformAdmin && defaultTenantID == "" {
				tenantRepo := repository.NewTenantRepository()
				tenants, _ := tenantRepo.GetUserTenants(ctx, user.ID, "")
				if len(tenants) > 0 {
					defaultTenantID = tenants[0].ID.String()
				}
			}
		}
	}

	statusCode := http.StatusOK
	if status == "failed" {
		statusCode = http.StatusUnauthorized
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if isPlatformAdmin {
		// 平台管理员登录 — 写入 platform_audit_logs
		platformLog := &model.PlatformAuditLog{
			UserID:         userID,
			Username:       username,
			IPAddress:      ipAddress,
			UserAgent:      userAgent,
			Category:       "login",
			Action:         "login",
			ResourceType:   "auth",
			RequestMethod:  "POST",
			RequestPath:    "/api/v1/auth/login",
			ResponseStatus: &statusCode,
			Status:         status,
			ErrorMessage:   errorMsg,
			CreatedAt:      createdAt,
		}
		if err := h.platformAuditRepo.Create(ctx, platformLog); err != nil {
			zap.L().Error("平台登录审计日志写入失败", zap.Error(err))
		}
	} else {
		// 租户用户登录 — 写入 audit_logs（设置 TenantID）
		auditLog := &model.AuditLog{
			UserID:         userID,
			Username:       username,
			IPAddress:      ipAddress,
			UserAgent:      userAgent,
			Category:       "login",
			Action:         "login",
			ResourceType:   "auth",
			RequestMethod:  "POST",
			RequestPath:    "/api/v1/auth/login",
			ResponseStatus: &statusCode,
			Status:         status,
			ErrorMessage:   errorMsg,
			CreatedAt:      createdAt,
		}
		// 设置租户 ID（确保登录日志能被按租户过滤）
		if defaultTenantID != "" {
			if tid, err := uuid.Parse(defaultTenantID); err == nil {
				auditLog.TenantID = &tid
			}
		}
		if err := h.auditRepo.Create(ctx, auditLog); err != nil {
			zap.L().Error("登录审计日志写入失败", zap.Error(err))
		}
	}
}

// Logout 用户登出
func (h *AuthHandler) Logout(c *gin.Context) {
	clientIP := middleware.NormalizeIP(c.ClientIP())
	userAgent := c.Request.UserAgent()
	startTime := time.Now()

	// 提取用户信息（在 token 失效前）
	userIDStr := middleware.GetUserID(c)
	username := middleware.GetUsername(c)
	isPlatformAdmin := middleware.IsPlatformAdmin(c)
	// 提取当前租户 ID（登出日志需要 tenant_id）
	tenantID := repository.TenantIDFromContext(c.Request.Context())

	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 {
		tokenString := authHeader[7:]
		claims, err := h.jwtSvc.ValidateToken(tokenString)
		if err == nil {
			_ = h.authSvc.Logout(c.Request.Context(), claims.ID, claims.ExpiresAt.Time)
		}
	}

	// 异步写入登出审计日志
	go h.writeLogoutAuditLog(userIDStr, username, clientIP, userAgent, startTime, isPlatformAdmin, tenantID)

	response.Message(c, "登出成功")
}

// writeLogoutAuditLog 异步写入登出审计日志
func (h *AuthHandler) writeLogoutAuditLog(userIDStr, username, ipAddress, userAgent string, createdAt time.Time, isPlatformAdmin bool, tenantID uuid.UUID) {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("登出审计日志记录失败 (panic)", zap.Any("error", r))
		}
	}()

	var userID *uuid.UUID
	if userIDStr != "" {
		if uid, err := uuid.Parse(userIDStr); err == nil {
			userID = &uid
		}
	}

	statusCode := http.StatusOK
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if isPlatformAdmin {
		platformLog := &model.PlatformAuditLog{
			UserID:         userID,
			Username:       username,
			IPAddress:      ipAddress,
			UserAgent:      userAgent,
			Category:       "login",
			Action:         "logout",
			ResourceType:   "auth-logout",
			RequestMethod:  "POST",
			RequestPath:    "/api/v1/auth/logout",
			ResponseStatus: &statusCode,
			Status:         "success",
			CreatedAt:      createdAt,
		}
		if err := h.platformAuditRepo.Create(ctx, platformLog); err != nil {
			zap.L().Error("平台登出审计日志写入失败", zap.Error(err))
		}
	} else {
		auditLog := &model.AuditLog{
			UserID:         userID,
			Username:       username,
			IPAddress:      ipAddress,
			UserAgent:      userAgent,
			Category:       "login",
			Action:         "logout",
			ResourceType:   "auth-logout",
			RequestMethod:  "POST",
			RequestPath:    "/api/v1/auth/logout",
			ResponseStatus: &statusCode,
			Status:         "success",
			CreatedAt:      createdAt,
		}
		// 设置租户 ID（确保登出日志能被按租户过滤）
		if tenantID != uuid.Nil {
			auditLog.TenantID = &tenantID
		}
		if err := h.auditRepo.Create(ctx, auditLog); err != nil {
			zap.L().Error("登出审计日志写入失败", zap.Error(err))
		}
	}
}

// RefreshToken 刷新令牌
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	userID, err := h.jwtSvc.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, "无效或过期的刷新令牌")
		return
	}

	uid, _ := uuid.Parse(userID)
	userInfo, err := h.authSvc.GetCurrentUser(c.Request.Context(), uid)
	if err != nil {
		response.Unauthorized(c, "用户不存在")
		return
	}

	// 🔒 禁用用户不能刷新 token
	userRepo := repository.NewUserRepository()
	user, userErr := userRepo.GetByID(c.Request.Context(), uid)
	if userErr != nil || user.Status != "active" {
		response.Unauthorized(c, "账户已被禁用")
		return
	}

	// 查询用户所属租户（和 Login 一样；平台管理员返回空）
	tenantRepo := repository.NewTenantRepository()
	var tenants []model.Tenant
	if !userInfo.IsPlatformAdmin {
		tenants, err = tenantRepo.GetUserTenants(c.Request.Context(), uid, "")
		if err != nil {
			response.InternalError(c, "获取租户信息失败")
			return
		}
	}

	tenantBriefs := make([]authService.TenantBrief, len(tenants))
	tenantIDs := make([]string, len(tenants))
	for i, tenant := range tenants {
		tenantBriefs[i] = authService.TenantBrief{
			ID:   tenant.ID.String(),
			Name: tenant.Name,
			Code: tenant.Code,
		}
		tenantIDs[i] = tenant.ID.String()
	}

	defaultTenantID := ""
	if len(tenants) > 0 {
		defaultTenantID = tenants[0].ID.String()
	}

	// 生成新 Token（携带租户信息）
	var tokenOpts []func(*jwt.Claims)
	if userInfo.IsPlatformAdmin {
		tokenOpts = append(tokenOpts, func(c *jwt.Claims) {
			c.IsPlatformAdmin = true
		})
	}
	tokenOpts = append(tokenOpts, func(c *jwt.Claims) {
		c.TenantIDs = tenantIDs
		c.DefaultTenantID = defaultTenantID
	})

	tokenPair, err := h.jwtSvc.GenerateTokenPair(userID, userInfo.Username, userInfo.Roles, userInfo.Permissions, tokenOpts...)
	if err != nil {
		response.InternalError(c, "生成令牌失败")
		return
	}

	// 刷新 token 响应保持原格式
	c.JSON(http.StatusOK, authService.LoginResponse{
		AccessToken:     tokenPair.AccessToken,
		RefreshToken:    tokenPair.RefreshToken,
		TokenType:       tokenPair.TokenType,
		ExpiresIn:       tokenPair.ExpiresIn,
		User:            *userInfo,
		Tenants:         tenantBriefs,
		CurrentTenantID: defaultTenantID,
	})
}

// GetCurrentUser 获取当前用户信息
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	userInfo, err := h.authSvc.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	// 🔒 权限覆盖逻辑（优先级从高到低）：
	// 1. Impersonation 模式 → 使用中间件已设置的 impersonation_accessor 角色权限
	//    （无论是 platform_admin 还是其他平台角色，提权时统一受限）
	// 2. 普通用户 + 有租户上下文 → 使用当前租户的角色和权限
	// 3. 平台管理员非提权 / 无租户 → 保留 service 层返回的平台角色权限

	if middleware.IsImpersonating(c) {
		// Impersonation 模式：中间件已用 impersonation_accessor 覆盖了 PermissionsKey
		if perms := middleware.GetPermissions(c); len(perms) > 0 {
			userInfo.Permissions = perms
		}
	} else if !userInfo.IsPlatformAdmin {
		// 普通用户：获取当前租户的精确权限和角色
		if tenantIDStr, exists := c.Get(middleware.TenantIDKey); exists && tenantIDStr != nil {
			if tid, parseErr := uuid.Parse(tenantIDStr.(string)); parseErr == nil {
				permRepo := repository.NewPermissionRepository()
				if tenantPerms, permErr := permRepo.GetTenantPermissionCodes(c.Request.Context(), userID, tid); permErr == nil {
					userInfo.Permissions = tenantPerms
				}
				tenantRepo := repository.NewTenantRepository()
				if tenantRoles, roleErr := tenantRepo.GetUserTenantRoles(c.Request.Context(), userID, tid); roleErr == nil {
					roleNames := make([]string, len(tenantRoles))
					for i, r := range tenantRoles {
						roleNames[i] = r.Name
					}
					userInfo.Roles = roleNames
				}
			}
		}
	}
	// 平台管理员非提权：保留 service 层返回的权限（platform_admin → "*"，其他 → 各自平台权限）

	response.Success(c, userInfo)
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req authService.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := h.authSvc.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	response.Message(c, "密码修改成功")
}

// GetProfile 获取当前用户详细信息（个人中心使用）
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	profile, err := h.authSvc.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	response.Success(c, profile)
}

// UpdateProfile 更新个人信息
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := h.authSvc.UpdateProfile(c.Request.Context(), userID, req.DisplayName, req.Email, req.Phone); err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	response.Message(c, "更新成功")
}

// UpdateProfileRequest 更新个人资料请求
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

// ==================== 个人中心：登录历史 & 操作记录 ====================

// LoginHistoryItem 登录历史条目
type LoginHistoryItem struct {
	ID           uuid.UUID `json:"id"`
	Action       string    `json:"action"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProfileActivityItem 操作记录条目
type ProfileActivityItem struct {
	ID           uuid.UUID `json:"id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceName string    `json:"resource_name,omitempty"`
	Status       string    `json:"status"`
	IPAddress    string    `json:"ip_address"`
	CreatedAt    time.Time `json:"created_at"`
}

// GetLoginHistory 获取当前用户的登录历史
// GET /api/v1/auth/profile/login-history?limit=10
func (h *AuthHandler) GetLoginHistory(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	var items []LoginHistoryItem

	if middleware.IsPlatformAdmin(c) {
		// 平台用户 → 查 platform_audit_logs
		logs, err := h.platformAuditRepo.GetUserLoginHistory(c.Request.Context(), userID, limit)
		if err != nil {
			response.InternalError(c, "获取登录历史失败: "+err.Error())
			return
		}
		for _, log := range logs {
			items = append(items, LoginHistoryItem{
				ID:           log.ID,
				Action:       log.Action,
				IPAddress:    log.IPAddress,
				UserAgent:    log.UserAgent,
				Status:       log.Status,
				ErrorMessage: log.ErrorMessage,
				CreatedAt:    log.CreatedAt,
			})
		}
	} else {
		// 租户用户 → 查 audit_logs（登录是全局操作，不按租户过滤）
		logs, err := h.auditRepo.GetUserLoginHistory(c.Request.Context(), userID, uuid.Nil, limit)
		if err != nil {
			response.InternalError(c, "获取登录历史失败: "+err.Error())
			return
		}
		for _, log := range logs {
			items = append(items, LoginHistoryItem{
				ID:           log.ID,
				Action:       log.Action,
				IPAddress:    log.IPAddress,
				UserAgent:    log.UserAgent,
				Status:       log.Status,
				ErrorMessage: log.ErrorMessage,
				CreatedAt:    log.CreatedAt,
			})
		}
	}

	if items == nil {
		items = []LoginHistoryItem{}
	}

	response.Success(c, map[string]interface{}{
		"items": items,
	})
}

// GetProfileActivities 获取当前用户的操作记录（排除 login/logout）
// GET /api/v1/auth/profile/activities?limit=15
func (h *AuthHandler) GetProfileActivities(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	limit := 15
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	var items []ProfileActivityItem

	if middleware.IsPlatformAdmin(c) {
		// 平台用户 → 查 platform_audit_logs
		logs, err := h.platformAuditRepo.GetUserActivities(c.Request.Context(), userID, limit)
		if err != nil {
			response.InternalError(c, "获取操作记录失败: "+err.Error())
			return
		}
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
	} else {
		// 租户用户 → 查 audit_logs（按当前租户过滤）
		tenantID := repository.TenantIDFromContext(c.Request.Context())
		logs, err := h.auditRepo.GetUserActivities(c.Request.Context(), userID, tenantID, limit)
		if err != nil {
			response.InternalError(c, "获取操作记录失败: "+err.Error())
			return
		}
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
	}

	if items == nil {
		items = []ProfileActivityItem{}
	}

	response.Success(c, map[string]interface{}{
		"items": items,
	})
}
