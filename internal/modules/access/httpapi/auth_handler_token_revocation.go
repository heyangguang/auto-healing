package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	authjwt "github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/gin-gonic/gin"
	golangjwt "github.com/golang-jwt/jwt/v5"
)

type logoutTokens struct {
	access  *authjwt.Claims
	refresh *golangjwt.RegisteredClaims
}

func (h *AuthHandler) revokeAuthTokens(c *gin.Context, refreshToken, userID string) error {
	tokens, err := h.resolveLogoutTokens(c, refreshToken, userID)
	if err != nil {
		return err
	}
	if err := h.blacklistLogoutTokens(c.Request.Context(), tokens); err != nil {
		return err
	}
	return nil
}

func (h *AuthHandler) resolveLogoutTokens(c *gin.Context, refreshToken, userID string) (logoutTokens, error) {
	accessClaims, err := h.resolveAccessTokenClaims(c)
	if err != nil {
		return logoutTokens{}, err
	}
	refreshClaims, err := h.resolveRefreshTokenClaims(c.Request.Context(), refreshToken, userID)
	if err != nil {
		return logoutTokens{}, err
	}
	if refreshClaims != nil {
		if requiresLegacyTokenRevocation(accessClaims) {
			return logoutTokens{}, errLogoutLegacyRefreshUnsupported
		}
		if refreshClaims.ID != accessClaims.ID {
			return logoutTokens{}, errLogoutRefreshTokenSessionMismatch
		}
	}
	return logoutTokens{access: accessClaims, refresh: refreshClaims}, nil
}

func (h *AuthHandler) resolveAccessTokenClaims(c *gin.Context) (*authjwt.Claims, error) {
	authHeader := c.GetHeader(middleware.AuthorizationHeader)
	if !strings.HasPrefix(authHeader, middleware.BearerPrefix) {
		return nil, fmt.Errorf("缺少当前访问令牌")
	}
	token := strings.TrimPrefix(authHeader, middleware.BearerPrefix)
	claims, err := h.jwtSvc.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("解析当前访问令牌失败: %w", err)
	}
	return claims, nil
}

func (h *AuthHandler) resolveRefreshTokenClaims(ctx context.Context, refreshToken, userID string) (*golangjwt.RegisteredClaims, error) {
	if refreshToken == "" {
		return nil, nil
	}
	refreshClaims, err := h.jwtSvc.ValidateRefreshTokenContext(ctx, refreshToken)
	if err != nil {
		return nil, mapLogoutRefreshTokenError(err)
	}
	if refreshClaims.Subject != userID {
		return nil, errLogoutRefreshTokenUserMismatch
	}
	return refreshClaims, nil
}

func (h *AuthHandler) blacklistLogoutTokens(ctx context.Context, tokens logoutTokens) error {
	if requiresLegacyTokenRevocation(tokens.access) {
		return h.blacklistLegacyLogoutTokens(ctx, tokens)
	}
	if err := h.blacklistSessionClaims(ctx, tokens.access, tokens.refresh); err != nil {
		return err
	}
	return nil
}

func (h *AuthHandler) blacklistLegacyLogoutTokens(ctx context.Context, tokens logoutTokens) error {
	if err := h.blacklistRefreshClaims(ctx, tokens.refresh); err != nil {
		return err
	}
	if err := h.blacklistAccessClaims(ctx, tokens.access); err != nil {
		return err
	}
	return nil
}

func (h *AuthHandler) blacklistSessionClaims(ctx context.Context, accessClaims *authjwt.Claims, refreshClaims *golangjwt.RegisteredClaims) error {
	if accessClaims == nil {
		return nil
	}
	sessionExp, err := accessSessionExpiry(accessClaims)
	if err != nil {
		return err
	}
	if refreshClaims != nil && refreshClaims.ExpiresAt != nil && refreshClaims.ExpiresAt.Time.After(sessionExp) {
		sessionExp = refreshClaims.ExpiresAt.Time
	}
	if err := h.authSvc.Logout(ctx, accessClaims.ID, sessionExp); err != nil {
		return fmt.Errorf("撤销当前会话失败: %w", err)
	}
	return nil
}

func (h *AuthHandler) blacklistAccessClaims(ctx context.Context, claims *authjwt.Claims) error {
	if claims == nil {
		return nil
	}
	tokenExp, err := accessTokenExpiry(claims)
	if err != nil {
		return err
	}
	if err := h.authSvc.Logout(ctx, claims.ID, tokenExp); err != nil {
		return fmt.Errorf("撤销访问令牌失败: %w", err)
	}
	return nil
}

func (h *AuthHandler) blacklistRefreshClaims(ctx context.Context, claims *golangjwt.RegisteredClaims) error {
	if claims == nil {
		return nil
	}
	if err := h.authSvc.Logout(ctx, claims.ID, claims.ExpiresAt.Time); err != nil {
		return fmt.Errorf("撤销刷新令牌失败: %w", err)
	}
	return nil
}

func (h *AuthHandler) revokeRefreshToken(ctx context.Context, refreshToken, userID string) error {
	refreshClaims, err := h.resolveRefreshTokenClaims(ctx, refreshToken, userID)
	if err != nil {
		return err
	}
	return h.blacklistRefreshClaims(ctx, refreshClaims)
}

func mapLogoutRefreshTokenError(err error) error {
	switch {
	case errors.Is(err, authjwt.ErrExpiredToken):
		return errLogoutRefreshTokenExpired
	case errors.Is(err, authjwt.ErrBlacklistLookupFailed):
		return authjwt.ErrBlacklistLookupFailed
	default:
		return errLogoutRefreshTokenInvalid
	}
}

func accessSessionExpiry(claims *authjwt.Claims) (time.Time, error) {
	if claims == nil {
		return time.Time{}, errLogoutSessionMetadataMissing
	}
	if claims.SessionExpiresAt > 0 {
		return time.Unix(claims.SessionExpiresAt, 0).UTC(), nil
	}
	return time.Time{}, errLogoutSessionMetadataMissing
}

func accessTokenExpiry(claims *authjwt.Claims) (time.Time, error) {
	if claims == nil || claims.ExpiresAt == nil {
		return time.Time{}, errLogoutSessionMetadataMissing
	}
	return claims.ExpiresAt.Time, nil
}

func requiresLegacyTokenRevocation(claims *authjwt.Claims) bool {
	return claims != nil && claims.SessionExpiresAt == 0
}
