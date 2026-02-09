package handler

import (
	"net/http"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/pkg/response"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authSvc *authService.Service
	jwtSvc  *jwt.Service
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
		authSvc: authService.NewService(jwtSvc),
		jwtSvc:  jwtSvc,
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

	resp, err := h.authSvc.Login(c.Request.Context(), &req, c.ClientIP())
	if err != nil {
		response.Unauthorized(c, ToBusinessError(err))
		return
	}

	// 登录响应保持原格式（包含 token 字段）
	c.JSON(http.StatusOK, resp)
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

	tokenPair, err := h.jwtSvc.GenerateTokenPair(userID, userInfo.Username, userInfo.Roles, userInfo.Permissions)
	if err != nil {
		response.InternalError(c, "生成令牌失败")
		return
	}

	// 刷新 token 响应保持原格式
	c.JSON(http.StatusOK, authService.LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    tokenPair.TokenType,
		ExpiresIn:    tokenPair.ExpiresIn,
		User:         *userInfo,
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
