package jwt

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken           = errors.New("invalid token")
	ErrExpiredToken           = errors.New("token has expired")
	ErrBlacklistStoreRequired = errors.New("blacklist store is required")
	ErrBlacklistLookupFailed  = errors.New("blacklist lookup failed")
)

const (
	tokenAudienceAccess  = "access"
	tokenAudienceRefresh = "refresh"
)

// Claims JWT声明
type Claims struct {
	jwt.RegisteredClaims
	Username         string   `json:"username"`
	Roles            []string `json:"roles"`
	Permissions      []string `json:"permissions"`
	IsPlatformAdmin  bool     `json:"is_platform_admin,omitempty"`
	TenantIDs        []string `json:"tenant_ids,omitempty"`        // 用户所属的租户列表
	DefaultTenantID  string   `json:"default_tenant_id,omitempty"` // 用户的默认租户（第一个）
	SessionExpiresAt int64    `json:"session_expires_at,omitempty"`
}

// Service JWT服务
type Service struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	issuer          string
	blacklist       BlacklistStore
}

// BlacklistStore Token黑名单存储接口
type BlacklistStore interface {
	Add(ctx context.Context, jti string, exp time.Time) error
	Exists(ctx context.Context, jti string) (bool, error)
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// Config JWT配置
type Config struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
}

// NewService 创建JWT服务
func NewService(cfg Config, blacklist BlacklistStore) *Service {
	if blacklist == nil {
		panic(ErrBlacklistStoreRequired)
	}
	return &Service{
		secret:          []byte(cfg.Secret),
		accessTokenTTL:  cfg.AccessTokenTTL,
		refreshTokenTTL: cfg.RefreshTokenTTL,
		issuer:          cfg.Issuer,
		blacklist:       blacklist,
	}
}

// GenerateTokenPair 生成令牌对
func (s *Service) GenerateTokenPair(userID, username string, roles, permissions []string, opts ...func(*Claims)) (*TokenPair, error) {
	now := time.Now()
	sessionID := uuid.New().String()
	sessionExpiresAt := now.Add(s.refreshTokenTTL)

	// 生成 Access Token
	accessClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			Subject:   userID,
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{tokenAudienceAccess},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTokenTTL)),
		},
		Username:         username,
		Roles:            roles,
		Permissions:      permissions,
		SessionExpiresAt: sessionExpiresAt.Unix(),
	}

	// 应用可选配置（如 IsPlatformAdmin）
	for _, opt := range opts {
		opt(&accessClaims)
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(s.secret)
	if err != nil {
		return nil, err
	}

	// 生成 Refresh Token
	refreshClaims := jwt.RegisteredClaims{
		ID:        sessionID,
		Subject:   userID,
		Issuer:    s.issuer,
		Audience:  jwt.ClaimStrings{tokenAudienceRefresh},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(sessionExpiresAt),
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(s.secret)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
	}, nil
}

// ValidateToken 验证Token
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || !hasAudience(claims.Audience, tokenAudienceAccess) || claims.Issuer != s.issuer {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ValidateRefreshToken 验证刷新Token
func (s *Service) ValidateRefreshToken(tokenString string) (*jwt.RegisteredClaims, error) {
	return s.ValidateRefreshTokenContext(context.Background(), tokenString)
}

func (s *Service) ValidateRefreshTokenContext(ctx context.Context, tokenString string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid || !hasAudience(claims.Audience, tokenAudienceRefresh) || claims.Issuer != s.issuer {
		return nil, ErrInvalidToken
	}
	isBlacklisted, blacklistErr := s.IsBlacklisted(ctx, claims.ID)
	if blacklistErr != nil {
		return nil, ErrBlacklistLookupFailed
	}
	if isBlacklisted {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// Blacklist 将Token加入黑名单
func (s *Service) Blacklist(ctx context.Context, jti string, exp time.Time) error {
	return s.blacklist.Add(ctx, jti, exp)
}

// IsBlacklisted 检查Token是否在黑名单中
func (s *Service) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	return s.blacklist.Exists(ctx, jti)
}

// GetAccessTokenTTL 获取访问令牌有效期
func (s *Service) GetAccessTokenTTL() time.Duration {
	return s.accessTokenTTL
}

func hasAudience(audiences jwt.ClaimStrings, expected string) bool {
	for _, audience := range audiences {
		if audience == expected {
			return true
		}
	}
	return false
}
