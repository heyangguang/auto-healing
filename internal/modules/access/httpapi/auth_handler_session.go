package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/pkg/response"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Login 用户登录
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
		platformlifecycle.Go(func(rootCtx context.Context) {
			h.writeLoginAuditLog(rootCtx, nil, req.Username, clientIP, userAgent, "failed", loginAuditErrorMessage(err), startTime, loginFailureStatusCode(err), false, "")
		})
		if isLoginUnauthorizedError(err) {
			response.Unauthorized(c, ToBusinessError(err))
			return
		}
		respondInternalError(c, "AUTH", "登录失败", err)
		return
	}

	userID := resp.User.ID
	platformlifecycle.Go(func(rootCtx context.Context) {
		h.writeLoginAuditLog(rootCtx, &userID, resp.User.Username, clientIP, userAgent, "success", "", startTime, http.StatusOK, resp.User.IsPlatformAdmin, resp.CurrentTenantID)
	})
	response.Success(c, resp)
}

// Logout 用户登出
func (h *AuthHandler) Logout(c *gin.Context) {
	clientIP := middleware.NormalizeIP(c.ClientIP())
	userAgent := c.Request.UserAgent()
	startTime := time.Now()

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
	}

	userIDStr := middleware.GetUserID(c)
	username := middleware.GetUsername(c)
	isPlatformAdmin := middleware.IsPlatformAdmin(c)
	tenantID := currentTenantOrNil(c)

	if err := h.revokeAuthTokens(c, req.RefreshToken, userIDStr); err != nil {
		if isLogoutClientError(err) {
			response.BadRequest(c, err.Error())
			return
		}
		respondInternalError(c, "AUTH", "登出失败", err)
		return
	}
	platformlifecycle.Go(func(rootCtx context.Context) {
		h.writeLogoutAuditLog(rootCtx, userIDStr, username, clientIP, userAgent, startTime, isPlatformAdmin, tenantID)
	})
	response.Message(c, "登出成功")
}

var (
	errLogoutRefreshTokenInvalid         = errors.New("刷新令牌无效")
	errLogoutRefreshTokenExpired         = errors.New("刷新令牌已过期")
	errLogoutRefreshTokenUserMismatch    = errors.New("刷新令牌与当前用户不匹配")
	errLogoutRefreshTokenSessionMismatch = errors.New("刷新令牌与当前会话不匹配")
	errLogoutLegacyRefreshUnsupported    = errors.New("旧版会话同时注销刷新令牌不安全，请重新登录后再登出")
	errLogoutSessionMetadataMissing      = errors.New("当前会话不支持原子登出，请重新登录后重试")
	errRefreshTokenInvalid               = errors.New("无效的刷新令牌")
	errRefreshUserInactive               = errors.New("账户已被禁用")
)

// RefreshToken 刷新令牌
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	refreshClaims, err := h.jwtSvc.ValidateRefreshTokenContext(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, jwt.ErrBlacklistLookupFailed) {
			response.InternalError(c, "刷新令牌状态校验失败")
			return
		}
		response.Unauthorized(c, "无效或过期的刷新令牌")
		return
	}

	userID, userInfo, err := h.refreshUserInfo(c, refreshClaims.Subject)
	if err != nil {
		respondRefreshUserInfoError(c, err)
		return
	}
	tenants, err := h.refreshUserTenants(c, userInfo, userID)
	if err != nil {
		response.InternalError(c, "获取租户信息失败")
		return
	}

	tokenPair, defaultTenantID, tenantBriefs, err := h.issueRefreshTokenPair(userInfo, refreshClaims.Subject, tenants)
	if err != nil {
		response.InternalError(c, "生成令牌失败")
		return
	}
	if err := h.blacklistRefreshClaims(c.Request.Context(), refreshClaims); err != nil {
		response.InternalError(c, "刷新令牌轮换失败")
		return
	}
	response.Success(c, authService.LoginResponse{
		AccessToken:     tokenPair.AccessToken,
		RefreshToken:    tokenPair.RefreshToken,
		TokenType:       tokenPair.TokenType,
		ExpiresIn:       tokenPair.ExpiresIn,
		User:            *userInfo,
		Tenants:         tenantBriefs,
		CurrentTenantID: defaultTenantID,
	})
}

func (h *AuthHandler) refreshUserInfo(c *gin.Context, userIDStr string) (uuid.UUID, *authService.UserInfo, error) {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, nil, errRefreshTokenInvalid
	}
	userInfo, err := h.authSvc.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, accessrepo.ErrUserNotFound) {
			return uuid.Nil, nil, accessrepo.ErrUserNotFound
		}
		return uuid.Nil, nil, fmt.Errorf("加载刷新用户信息失败: %w", err)
	}
	user, userErr := h.userRepo.GetByID(c.Request.Context(), userID)
	if userErr != nil {
		if errors.Is(userErr, accessrepo.ErrUserNotFound) {
			return uuid.Nil, nil, accessrepo.ErrUserNotFound
		}
		return uuid.Nil, nil, fmt.Errorf("校验刷新用户状态失败: %w", userErr)
	}
	if user.Status != "active" {
		return uuid.Nil, nil, errRefreshUserInactive
	}
	return userID, userInfo, nil
}

func respondRefreshUserInfoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errRefreshTokenInvalid),
		errors.Is(err, accessrepo.ErrUserNotFound),
		errors.Is(err, errRefreshUserInactive):
		response.Unauthorized(c, err.Error())
	default:
		respondInternalError(c, "AUTH", "刷新令牌失败", err)
	}
}

func (h *AuthHandler) refreshUserTenants(c *gin.Context, userInfo *authService.UserInfo, userID uuid.UUID) ([]model.Tenant, error) {
	if userInfo.IsPlatformAdmin {
		return nil, nil
	}
	return h.tenantRepo.GetUserTenants(c.Request.Context(), userID, "")
}

func (h *AuthHandler) issueRefreshTokenPair(userInfo *authService.UserInfo, userID string, tenants []model.Tenant) (*jwt.TokenPair, string, []authService.TenantBrief, error) {
	tenantBriefs := make([]authService.TenantBrief, len(tenants))
	tenantIDs := make([]string, len(tenants))
	for i, tenant := range tenants {
		tenantBriefs[i] = authService.TenantBrief{ID: tenant.ID.String(), Name: tenant.Name, Code: tenant.Code}
		tenantIDs[i] = tenant.ID.String()
	}

	defaultTenantID := ""
	if len(tenants) > 0 {
		defaultTenantID = tenants[0].ID.String()
	}

	var tokenOpts []func(*jwt.Claims)
	if userInfo.IsPlatformAdmin {
		tokenOpts = append(tokenOpts, func(claims *jwt.Claims) { claims.IsPlatformAdmin = true })
	}
	tokenOpts = append(tokenOpts, func(claims *jwt.Claims) {
		claims.TenantIDs = tenantIDs
		claims.DefaultTenantID = defaultTenantID
	})

	tokenPair, err := h.jwtSvc.GenerateTokenPair(userID, userInfo.Username, userInfo.Roles, userInfo.Permissions, tokenOpts...)
	if err != nil {
		return nil, "", nil, err
	}
	return tokenPair, defaultTenantID, tenantBriefs, nil
}
