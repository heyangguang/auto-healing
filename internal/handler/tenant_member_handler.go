package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/service"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==================== 租户成员 & 邀请管理 Handler ====================
// 补充 TenantHandler 的成员管理能力：添加/移除成员、邀请用户

// ==================== DTO ====================

type addMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	RoleID string `json:"role_id" binding:"required"`
}

type inviteToTenantRequest struct {
	Email     string `json:"email" binding:"required,email"`
	RoleID    string `json:"role_id" binding:"required"`
	SendEmail bool   `json:"send_email"` // 是否发送邮件（可选）
}

type inviteResponse struct {
	Invitation    *model.TenantInvitation `json:"invitation"`
	InvitationURL string                  `json:"invitation_url"`
	EmailSent     bool                    `json:"email_sent"`
	EmailMessage  string                  `json:"email_message,omitempty"` // 邮件状态提示
}

// ==================== 添加成员 ====================

// AddMember 添加已有用户到租户
// POST /api/v1/platform/tenants/:id/members
func (h *TenantHandler) AddMember(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：user_id 和 role_id 为必填")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return
	}

	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	// 验证租户存在
	tenant, err := h.repo.GetByID(c.Request.Context(), tenantID)
	if err != nil {
		response.NotFound(c, "租户不存在")
		return
	}
	if tenant.Status != model.TenantStatusActive {
		response.BadRequest(c, "租户已禁用，无法添加成员")
		return
	}

	// 验证用户存在
	targetUser, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	// 互斥校验：平台管理员不能加入租户
	for _, role := range targetUser.Roles {
		if role.Name == "platform_admin" {
			response.BadRequest(c, "平台管理员不能加入租户，请选择其他用户")
			return
		}
	}

	// 验证角色存在且为系统级租户角色
	role, err := h.roleRepo.GetByID(c.Request.Context(), roleID)
	if err != nil {
		response.BadRequest(c, "角色不存在")
		return
	}
	if !isValidTenantRole(role) {
		response.BadRequest(c, "只能分配系统级租户角色（如管理员、运维人员、只读用户等）")
		return
	}

	// 检查是否已在租户内
	existingMember, _ := h.repo.GetMember(c.Request.Context(), userID, tenantID)
	if existingMember != nil {
		response.Conflict(c, "该用户已是租户成员")
		return
	}

	// 添加成员
	if err := h.repo.AddMember(c.Request.Context(), userID, tenantID, roleID); err != nil {
		response.InternalError(c, "添加成员失败")
		return
	}

	response.Message(c, "成员添加成功")
}

// ==================== 移除成员 ====================

// RemoveMember 从租户移除成员
// DELETE /api/v1/platform/tenants/:id/members/:userId
func (h *TenantHandler) RemoveMember(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return
	}

	// 验证用户在租户内
	member, err := h.repo.GetMember(c.Request.Context(), userID, tenantID)
	if err != nil {
		response.NotFound(c, "该用户不属于此租户")
		return
	}

	// 安全校验：不允许移除最后一个 admin
	adminRole, err := h.roleRepo.GetByName(c.Request.Context(), "admin")
	if err == nil && member.RoleID == adminRole.ID {
		// 统计当前租户的 admin 数量
		members, err := h.repo.ListMembers(c.Request.Context(), tenantID)
		if err != nil {
			response.InternalError(c, "查询成员失败")
			return
		}
		adminCount := 0
		for _, m := range members {
			if m.RoleID == adminRole.ID {
				adminCount++
			}
		}
		if adminCount <= 1 {
			response.BadRequest(c, "不能移除最后一个管理员，请先设置其他管理员")
			return
		}
	}

	if err := h.repo.RemoveMember(c.Request.Context(), userID, tenantID); err != nil {
		response.InternalError(c, "移除成员失败")
		return
	}

	response.Message(c, "成员已移除")
}

// ==================== 邀请用户 ====================

// InviteToTenant 邀请用户加入租户
// POST /api/v1/platform/tenants/:id/invitations
func (h *TenantHandler) InviteToTenant(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	var req inviteToTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：email 和 role_id 为必填")
		return
	}

	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	// 验证租户存在
	tenant, err := h.repo.GetByID(c.Request.Context(), tenantID)
	if err != nil {
		response.NotFound(c, "租户不存在")
		return
	}
	if tenant.Status != model.TenantStatusActive {
		response.BadRequest(c, "租户已禁用")
		return
	}

	// 验证角色存在且为系统级租户角色
	role, err := h.roleRepo.GetByID(c.Request.Context(), roleID)
	if err != nil {
		response.BadRequest(c, "角色不存在")
		return
	}
	if !isValidTenantRole(role) {
		response.BadRequest(c, "只能分配系统级租户角色（如管理员、运维人员、只读用户等）")
		return
	}

	// 检查邮箱是否已有未处理邀请
	invRepo := repository.NewInvitationRepository()
	hasPending, _ := invRepo.CheckEmailPendingInTenant(c.Request.Context(), tenantID, req.Email)
	if hasPending {
		response.Conflict(c, "该邮箱已有待处理的邀请")
		return
	}

	// 检查邮箱是否已是用户且已在租户内
	existingUser, _ := h.userRepo.GetByEmail(c.Request.Context(), req.Email)
	if existingUser != nil {
		existingMember, _ := h.repo.GetMember(c.Request.Context(), existingUser.ID, tenantID)
		if existingMember != nil {
			response.Conflict(c, "该邮箱用户已是租户成员")
			return
		}
	}

	// 生成邀请 token
	token, tokenHash, err := generateInvitationToken()
	if err != nil {
		response.InternalError(c, "生成邀请令牌失败")
		return
	}

	// 获取过期天数（从平台设置读取）
	settingsRepo := repository.NewPlatformSettingsRepository()
	expireDays := settingsRepo.GetIntValue(c.Request.Context(), "email.invitation_expire_days", 7)

	// 获取邀请人 ID
	inviterID, _ := uuid.Parse(middleware.GetUserID(c))

	// 创建邀请记录
	invitation := &model.TenantInvitation{
		TenantID:  tenantID,
		Email:     req.Email,
		RoleID:    roleID,
		Token:     token,
		TokenHash: tokenHash,
		Status:    model.InvitationStatusPending,
		InvitedBy: inviterID,
		ExpiresAt: time.Now().AddDate(0, 0, expireDays),
	}

	if err := invRepo.Create(c.Request.Context(), invitation); err != nil {
		response.InternalError(c, "创建邀请失败")
		return
	}

	// 构建邀请链接
	// 优先使用平台设置的站点 URL
	baseURL := settingsRepo.GetStringValue(c.Request.Context(), "site.base_url", "")
	if baseURL == "" {
		// 回退：尝试从请求头推断
		baseURL = c.Request.Header.Get("Origin")
	}
	if baseURL == "" {
		baseURL = fmt.Sprintf("%s://%s", getScheme(c), c.Request.Host)
	}
	// 去掉末尾斜杠
	baseURL = strings.TrimRight(baseURL, "/")
	invitationURL := fmt.Sprintf("%s/user/register?token=%s", baseURL, token)

	// 重新加载邀请（包含关联数据）
	invitation, _ = invRepo.GetByID(c.Request.Context(), invitation.ID)

	// 尝试发送邮件
	resp := inviteResponse{
		Invitation:    invitation,
		InvitationURL: invitationURL,
		EmailSent:     false,
	}

	if req.SendEmail {
		emailSvc := service.NewPlatformEmailService()
		if emailSvc.IsConfigured(c.Request.Context()) {
			err := emailSvc.SendInvitationEmail(c.Request.Context(), req.Email, tenant.Name, role.DisplayName, invitationURL)
			if err != nil {
				resp.EmailMessage = fmt.Sprintf("邮件发送失败: %s。请手动复制链接发送给用户。", err.Error())
			} else {
				resp.EmailSent = true
				resp.EmailMessage = "邀请邮件已发送"
			}
		} else {
			resp.EmailMessage = "平台邮箱服务未配置，请在平台设置中配置 SMTP 参数，或手动复制链接发送给用户。"
		}
	}

	response.Created(c, resp)
}

// ListInvitations 查看租户邀请记录
// GET /api/v1/platform/tenants/:id/invitations?status=pending&page=1&page_size=20
func (h *TenantHandler) ListInvitations(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	invRepo := repository.NewInvitationRepository()

	// 先过期旧邀请
	invRepo.ExpireOldInvitations(c.Request.Context())

	invitations, total, err := invRepo.ListByTenant(c.Request.Context(), tenantID, status, page, pageSize)
	if err != nil {
		response.InternalError(c, "查询邀请记录失败")
		return
	}

	// 为 pending 状态的邀请构建邀请链接
	settingsRepo := repository.NewPlatformSettingsRepository()
	baseURL := settingsRepo.GetStringValue(c.Request.Context(), "site.base_url", "")
	if baseURL == "" {
		baseURL = c.Request.Header.Get("Origin")
	}
	if baseURL == "" {
		baseURL = fmt.Sprintf("%s://%s", getScheme(c), c.Request.Host)
	}
	baseURL = strings.TrimRight(baseURL, "/")

	for i := range invitations {
		if invitations[i].Status == model.InvitationStatusPending && invitations[i].Token != "" {
			invitations[i].InvitationURL = fmt.Sprintf("%s/user/register?token=%s", baseURL, invitations[i].Token)
		}
	}

	response.List(c, invitations, total, page, pageSize)
}

// CancelInvitation 取消邀请
// DELETE /api/v1/platform/tenants/:id/invitations/:invId
func (h *TenantHandler) CancelInvitation(c *gin.Context) {
	_, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	invID, err := uuid.Parse(c.Param("invId"))
	if err != nil {
		response.BadRequest(c, "无效的邀请 ID")
		return
	}

	invRepo := repository.NewInvitationRepository()
	inv, err := invRepo.GetByID(c.Request.Context(), invID)
	if err != nil {
		response.NotFound(c, "邀请不存在")
		return
	}

	if inv.Status != model.InvitationStatusPending {
		response.BadRequest(c, "只能取消待处理的邀请")
		return
	}

	if err := invRepo.UpdateStatus(c.Request.Context(), invID, model.InvitationStatusCancelled); err != nil {
		response.InternalError(c, "取消邀请失败")
		return
	}

	response.Message(c, "邀请已取消")
}

// ==================== 邀请注册（公开接口） ====================

// ValidateInvitation 验证邀请 token
// GET /api/v1/auth/invitation/:token
func ValidateInvitation(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		response.BadRequest(c, "邀请令牌不能为空")
		return
	}

	tokenHash := hashToken(token)
	invRepo := repository.NewInvitationRepository()

	// 先过期旧邀请
	invRepo.ExpireOldInvitations(c.Request.Context())

	inv, err := invRepo.GetByTokenHash(c.Request.Context(), tokenHash)
	if err != nil {
		response.NotFound(c, "邀请不存在或已过期")
		return
	}

	// 检查是否已过期
	if time.Now().After(inv.ExpiresAt) {
		invRepo.UpdateStatus(c.Request.Context(), inv.ID, model.InvitationStatusExpired)
		response.BadRequest(c, "邀请已过期")
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

// RegisterByInvitationRequest 邀请注册请求
type RegisterByInvitationRequest struct {
	Token       string `json:"token" binding:"required"`
	Username    string `json:"username" binding:"required,min=3,max=50"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name"`
}

// RegisterByInvitation 通过邀请注册
// POST /api/v1/auth/register
func RegisterByInvitation(authSvc *authService.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterByInvitationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
			return
		}

		tokenHash := hashToken(req.Token)
		invRepo := repository.NewInvitationRepository()

		// 先过期旧邀请
		invRepo.ExpireOldInvitations(c.Request.Context())

		// 查找邀请
		inv, err := invRepo.GetByTokenHash(c.Request.Context(), tokenHash)
		if err != nil {
			response.NotFound(c, "邀请不存在或已过期")
			return
		}

		// 再次检查过期
		if time.Now().After(inv.ExpiresAt) {
			invRepo.UpdateStatus(c.Request.Context(), inv.ID, model.InvitationStatusExpired)
			response.BadRequest(c, "邀请已过期")
			return
		}

		// 创建用户并关联到租户
		regReq := &authService.RegisterRequest{
			Username:    req.Username,
			Email:       inv.Email,
			Password:    req.Password,
			DisplayName: req.DisplayName,
			TenantID:    &inv.TenantID,
		}

		user, err := authSvc.Register(c.Request.Context(), regReq)
		if err != nil {
			response.BadRequest(c, ToBusinessError(err))
			return
		}

		// 更新邀请中指定的角色（Register 默认分配 viewer，这里要覆盖）
		tenantRepo := repository.NewTenantRepository()
		if err := tenantRepo.UpdateMemberRole(c.Request.Context(), user.ID, inv.TenantID, inv.RoleID); err != nil {
			// 不影响注册成功
			fmt.Printf("更新邀请角色失败: %v\n", err)
		}

		// 标记邀请为已接受
		invRepo.UpdateStatus(c.Request.Context(), inv.ID, model.InvitationStatusAccepted)

		response.Created(c, gin.H{
			"user":    user,
			"message": "注册成功，请登录",
		})
	}
}

// ==================== 辅助函数 ====================

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

// isValidTenantRole 校验角色是否为系统级租户角色
// 规则：is_system=true + 非 platform_ 前缀 + 非 impersonation_accessor
func isValidTenantRole(role *model.Role) bool {
	if role == nil {
		return false
	}
	if !role.IsSystem {
		return false
	}
	if strings.HasPrefix(role.Name, "platform_") {
		return false
	}
	if role.Name == "impersonation_accessor" {
		return false
	}
	return true
}

// 确保使用了导入的包（防止编译错误）
var _ = errors.New
