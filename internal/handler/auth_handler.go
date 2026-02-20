package handler

import (
	"context"
	"net/http"
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
		go h.writeLoginAuditLog(nil, req.Username, clientIP, userAgent, "failed", err.Error(), startTime, false)
		response.Unauthorized(c, ToBusinessError(err))
		return
	}

	// 登录成功 — 记录审计日志
	userID := resp.User.ID
	go h.writeLoginAuditLog(&userID, resp.User.Username, clientIP, userAgent, "success", "", startTime, resp.User.IsPlatformAdmin)

	// 登录响应保持原格式（包含 token 字段）
	c.JSON(http.StatusOK, resp)
}

// writeLoginAuditLog 异步写入登录审计日志
// isPlatformAdmin: 是否是平台管理员（决定写入哪张表）
func (h *AuthHandler) writeLoginAuditLog(userID *uuid.UUID, username, ipAddress, userAgent, status, errorMsg string, createdAt time.Time, isPlatformAdmin bool) {
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
			isPlatformAdmin = user.IsPlatformAdmin
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
		// 租户用户登录 — 写入 audit_logs
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
		if err := h.auditRepo.Create(ctx, auditLog); err != nil {
			zap.L().Error("登录审计日志写入失败", zap.Error(err))
		}
	}
}

// Logout 用户登出
func (h *AuthHandler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 {
		tokenString := authHeader[7:]
		claims, err := h.jwtSvc.ValidateToken(tokenString)
		if err == nil {
			_ = h.authSvc.Logout(c.Request.Context(), claims.ID, claims.ExpiresAt.Time)
		}
	}

	response.Message(c, "登出成功")
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

	// 查询用户所属租户（和 Login 一样）
	tenantRepo := repository.NewTenantRepository()
	tenants, err := tenantRepo.GetUserTenants(c.Request.Context(), uid, "")
	if err != nil {
		response.InternalError(c, "获取租户信息失败")
		return
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
