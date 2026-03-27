package httpapi

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	engagementservice "github.com/company/auto-healing/internal/modules/engagement/service"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InviteToTenant 邀请用户加入租户
func (h *TenantHandler) InviteToTenant(c *gin.Context) {
	tenantID, roleID, req, ok := parseTenantInvitationRequest(c)
	if !ok {
		return
	}

	tenant, role, ok := h.validateTenantInvitationRequest(c, tenantID, roleID, req.Email)
	if !ok {
		return
	}

	token, tokenHash, err := generateInvitationToken()
	if err != nil {
		response.InternalError(c, "生成邀请令牌失败")
		return
	}

	settingsRepo := repository.NewPlatformSettingsRepository()
	inviterID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		respondInternalError(c, "TENANT", "解析邀请人失败", err)
		return
	}
	invRepo := repository.NewInvitationRepository()
	invitation := &model.TenantInvitation{
		TenantID:  tenantID,
		Email:     req.Email,
		RoleID:    roleID,
		Token:     token,
		TokenHash: tokenHash,
		Status:    model.InvitationStatusPending,
		InvitedBy: inviterID,
		ExpiresAt: time.Now().AddDate(0, 0, settingsRepo.GetIntValue(c.Request.Context(), "email.invitation_expire_days", 7)),
	}
	if err := invRepo.Create(c.Request.Context(), invitation); err != nil {
		response.InternalError(c, "创建邀请失败")
		return
	}

	invitationURL := buildInvitationURL(c, settingsRepo, token)
	invitation, err = invRepo.GetByID(c.Request.Context(), invitation.ID)
	if err != nil {
		respondInvitationLookupError(c, "TENANT", "查询邀请记录失败", err)
		return
	}
	response.Created(c, h.buildInviteResponse(c, req.SendEmail, req.Email, tenant.Name, role.DisplayName, invitation, invitationURL))
}

func parseTenantInvitationRequest(c *gin.Context) (uuid.UUID, uuid.UUID, inviteToTenantRequest, bool) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return uuid.Nil, uuid.Nil, inviteToTenantRequest{}, false
	}

	var req inviteToTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：email 和 role_id 为必填")
		return uuid.Nil, uuid.Nil, inviteToTenantRequest{}, false
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return uuid.Nil, uuid.Nil, inviteToTenantRequest{}, false
	}
	return tenantID, roleID, req, true
}

func (h *TenantHandler) validateTenantInvitationRequest(c *gin.Context, tenantID, roleID uuid.UUID, email string) (*model.Tenant, *model.Role, bool) {
	tenant, err := h.repo.GetByID(c.Request.Context(), tenantID)
	if err != nil {
		respondTenantLookupError(c, err)
		return nil, nil, false
	}
	if tenant.Status != model.TenantStatusActive {
		response.BadRequest(c, "租户已禁用")
		return nil, nil, false
	}

	role, err := h.roleRepo.GetTenantRoleByID(c.Request.Context(), tenantID, roleID)
	if err != nil {
		response.BadRequest(c, "角色不存在")
		return nil, nil, false
	}
	if !isValidTenantRole(role) {
		response.BadRequest(c, "只能分配系统级租户角色（如管理员、运维人员、只读用户等）")
		return nil, nil, false
	}

	invRepo := repository.NewInvitationRepository()
	hasPending, err := invRepo.CheckEmailPendingInTenant(c.Request.Context(), tenantID, email)
	if err != nil {
		respondInternalError(c, "TENANT", "检查待处理邀请失败", err)
		return nil, nil, false
	}
	if hasPending {
		response.Conflict(c, "该邮箱已有待处理的邀请")
		return nil, nil, false
	}

	existingUser, err := h.userRepo.GetByEmail(c.Request.Context(), email)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		respondInternalError(c, "TENANT", "查询用户失败", err)
		return nil, nil, false
	}
	if existingUser != nil {
		if existingUser.IsPlatformAdmin {
			response.BadRequest(c, "平台管理员不能加入租户，请选择其他用户")
			return nil, nil, false
		}
		existingMember, err := h.repo.GetMember(c.Request.Context(), existingUser.ID, tenantID)
		if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
			respondInternalError(c, "TENANT", "查询成员关系失败", err)
			return nil, nil, false
		}
		if existingMember != nil {
			response.Conflict(c, "该邮箱用户已是租户成员")
			return nil, nil, false
		}
		response.Conflict(c, "该邮箱已注册，请直接添加成员")
		return nil, nil, false
	}
	return tenant, role, true
}

func buildInvitationURL(c *gin.Context, settingsRepo *repository.PlatformSettingsRepository, token string) string {
	baseURL := settingsRepo.GetStringValue(c.Request.Context(), "site.base_url", "")
	if baseURL == "" {
		baseURL = c.Request.Header.Get("Origin")
	}
	if baseURL == "" {
		baseURL = fmt.Sprintf("%s://%s", getScheme(c), c.Request.Host)
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/user/register?token=%s", baseURL, token)
}

func (h *TenantHandler) buildInviteResponse(c *gin.Context, sendEmail bool, email, tenantName, roleName string, invitation *model.TenantInvitation, invitationURL string) inviteResponse {
	resp := inviteResponse{
		Invitation:    invitation,
		InvitationURL: invitationURL,
		EmailSent:     false,
	}
	if !sendEmail {
		return resp
	}

	emailSvc := engagementservice.NewPlatformEmailService()
	if !emailSvc.IsConfigured(c.Request.Context()) {
		resp.EmailMessage = "平台邮箱服务未配置，请在平台设置中配置 SMTP 参数，或手动复制链接发送给用户。"
		return resp
	}
	if err := emailSvc.SendInvitationEmail(c.Request.Context(), email, tenantName, roleName, invitationURL); err != nil {
		resp.EmailMessage = fmt.Sprintf("邮件发送失败: %s。请手动复制链接发送给用户。", err.Error())
		return resp
	}
	resp.EmailSent = true
	resp.EmailMessage = "邀请邮件已发送"
	return resp
}

// ListInvitations 查看租户邀请记录
func (h *TenantHandler) ListInvitations(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}
	if _, err := h.repo.GetByID(c.Request.Context(), tenantID); err != nil {
		respondTenantLookupError(c, err)
		return
	}
	status := c.Query("status")
	page, pageSize := parsePagination(c, 20)

	invRepo := repository.NewInvitationRepository()
	if _, err := invRepo.ExpireOldInvitations(c.Request.Context()); err != nil {
		respondInternalError(c, "TENANT", "更新邀请过期状态失败", err)
		return
	}
	invitations, total, err := invRepo.ListByTenant(c.Request.Context(), tenantID, status, page, pageSize)
	if err != nil {
		response.InternalError(c, "查询邀请记录失败")
		return
	}

	settingsRepo := repository.NewPlatformSettingsRepository()
	baseURL := buildInvitationURL(c, settingsRepo, "")
	baseURL = strings.TrimSuffix(baseURL, "?token=")
	for i := range invitations {
		if invitations[i].Status == model.InvitationStatusPending && invitations[i].Token != "" {
			invitations[i].InvitationURL = fmt.Sprintf("%s?token=%s", baseURL, invitations[i].Token)
		}
	}
	response.List(c, invitations, total, page, pageSize)
}

// CancelInvitation 取消邀请
func (h *TenantHandler) CancelInvitation(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
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
		respondInvitationLookupError(c, "TENANT", "查询邀请失败", err)
		return
	}
	if !invitationBelongsToTenant(inv, tenantID) {
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

func invitationBelongsToTenant(inv *model.TenantInvitation, tenantID uuid.UUID) bool {
	return inv != nil && inv.TenantID == tenantID
}

func respondInvitationLookupError(c *gin.Context, module, message string, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.NotFound(c, "邀请不存在")
		return
	}
	respondInternalError(c, module, message, err)
}
