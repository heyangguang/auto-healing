package handler

import (
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
)

// ValidateInvitation 验证邀请 token
func ValidateInvitation(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		response.BadRequest(c, "邀请令牌不能为空")
		return
	}

	inv, ok := loadValidInvitation(c, token)
	if !ok {
		return
	}
	response.Success(c, gin.H{
		"id":          inv.ID,
		"email":       inv.Email,
		"tenant_name": inv.Tenant.Name,
		"tenant_code": inv.Tenant.Code,
		"role_name":   inv.Role.DisplayName,
		"expires_at":  inv.ExpiresAt,
	})
}

// RegisterByInvitation 通过邀请注册
func RegisterByInvitation(authSvc *authService.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterByInvitationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
			return
		}

		inv, ok := loadValidInvitation(c, req.Token)
		if !ok {
			return
		}

		user, err := authSvc.Register(c.Request.Context(), &authService.RegisterRequest{
			Username:    req.Username,
			Email:       inv.Email,
			Password:    req.Password,
			DisplayName: req.DisplayName,
			TenantID:    &inv.TenantID,
		})
		if err != nil {
			response.BadRequest(c, ToBusinessError(err))
			return
		}

		tenantRepo := repository.NewTenantRepository()
		if err := tenantRepo.UpdateMemberRole(c.Request.Context(), user.ID, inv.TenantID, inv.RoleID); err != nil {
			fmt.Printf("更新邀请角色失败: %v\n", err)
		}

		invRepo := repository.NewInvitationRepository()
		invRepo.UpdateStatus(c.Request.Context(), inv.ID, model.InvitationStatusAccepted)
		response.Created(c, gin.H{
			"user":    user,
			"message": "注册成功，请登录",
		})
	}
}

func loadValidInvitation(c *gin.Context, token string) (*model.TenantInvitation, bool) {
	invRepo := repository.NewInvitationRepository()
	invRepo.ExpireOldInvitations(c.Request.Context())

	inv, err := invRepo.GetByTokenHash(c.Request.Context(), hashToken(token))
	if err != nil {
		response.NotFound(c, "邀请不存在或已过期")
		return nil, false
	}
	if time.Now().After(inv.ExpiresAt) {
		invRepo.UpdateStatus(c.Request.Context(), inv.ID, model.InvitationStatusExpired)
		response.BadRequest(c, "邀请已过期")
		return nil, false
	}
	return inv, true
}

// getScheme 获取请求协议
func getScheme(c *gin.Context) string {
	if c.Request.TLS != nil {
		return "https"
	}
	if proto := c.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}
