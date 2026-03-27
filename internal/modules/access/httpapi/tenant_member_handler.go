package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/company/auto-healing/internal/model"
)

type addMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	RoleID string `json:"role_id" binding:"required"`
}

type inviteToTenantRequest struct {
	Email     string `json:"email" binding:"required,email"`
	RoleID    string `json:"role_id" binding:"required"`
	SendEmail bool   `json:"send_email"`
}

type inviteResponse struct {
	Invitation    *model.TenantInvitation `json:"invitation"`
	InvitationURL string                  `json:"invitation_url"`
	EmailSent     bool                    `json:"email_sent"`
	EmailMessage  string                  `json:"email_message,omitempty"`
}

// RegisterByInvitationRequest 邀请注册请求
type RegisterByInvitationRequest struct {
	Token       string `json:"token" binding:"required"`
	Username    string `json:"username" binding:"required,min=3,max=50"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name"`
}

// generateInvitationToken 生成安全的邀请 token
func generateInvitationToken() (token string, tokenHash string, err error) {
	bytes := make([]byte, 32)
	if _, err = rand.Read(bytes); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(bytes)
	tokenHash = hashToken(token)
	return
}

// hashToken 对 token 做 SHA256 哈希
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// isValidTenantRole 校验角色是否为系统级租户角色
func isValidTenantRole(role *model.Role) bool {
	if role == nil || !role.IsSystem {
		return false
	}
	if strings.HasPrefix(role.Name, "platform_") {
		return false
	}
	return role.Name != "impersonation_accessor"
}
