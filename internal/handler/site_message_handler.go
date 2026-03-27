package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	platformevents "github.com/company/auto-healing/internal/platform/events"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SiteMessageHandler 站内信处理器
type SiteMessageHandler struct {
	repo             *repository.SiteMessageRepository
	platformSettings *repository.PlatformSettingsRepository
	eventBus         *platformevents.MessageEventBus
	tenantRepo       *repository.TenantRepository
	userRepo         *repository.UserRepository
}

type SiteMessageHandlerDeps struct {
	SiteMessageRepo      *repository.SiteMessageRepository
	PlatformSettingsRepo *repository.PlatformSettingsRepository
	EventBus             *platformevents.MessageEventBus
	TenantRepo           *repository.TenantRepository
	UserRepo             *repository.UserRepository
}

// NewSiteMessageHandler 创建站内信处理器
func NewSiteMessageHandler() *SiteMessageHandler {
	return NewSiteMessageHandlerWithDeps(SiteMessageHandlerDeps{
		SiteMessageRepo:      repository.NewSiteMessageRepository(),
		PlatformSettingsRepo: repository.NewPlatformSettingsRepository(),
		EventBus:             platformevents.GetMessageEventBus(),
		TenantRepo:           repository.NewTenantRepository(),
		UserRepo:             repository.NewUserRepository(),
	})
}

func NewSiteMessageHandlerWithDeps(deps SiteMessageHandlerDeps) *SiteMessageHandler {
	return &SiteMessageHandler{
		repo:             deps.SiteMessageRepo,
		platformSettings: deps.PlatformSettingsRepo,
		eventBus:         deps.EventBus,
		tenantRepo:       deps.TenantRepo,
		userRepo:         deps.UserRepo,
	}
}

type createSiteMessageRequest struct {
	Category        string   `json:"category" binding:"required"`
	Title           string   `json:"title" binding:"required"`
	Content         string   `json:"content" binding:"required"`
	TargetTenantIDs []string `json:"target_tenant_ids"`
}

type markReadRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

type updateSiteMessageSettingsRequest struct {
	RetentionDays int `json:"retention_days" binding:"required,min=1,max=3650"`
}

type siteMessageSettingsResponse struct {
	RetentionDays int    `json:"retention_days"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

func (h *SiteMessageHandler) getUserCreatedAt(c *gin.Context, userID uuid.UUID) time.Time {
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		logger.Warn("获取用户创建时间失败: userID=%s, err=%v", userID, err)
		return time.Time{}
	}
	return user.CreatedAt
}

func parseCurrentUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return uuid.Nil, false
	}
	return userID, true
}

func (h *SiteMessageHandler) currentTenantContext(c *gin.Context, userID uuid.UUID) (uuid.UUID, time.Time, bool) {
	tenantID, ok := requireTenantID(c, "SITE_MESSAGE")
	if !ok {
		return uuid.Nil, time.Time{}, false
	}
	return tenantID, h.getUserCreatedAt(c, userID), true
}

func validSiteMessageCategory(category string) bool {
	for _, item := range model.AllSiteMessageCategories {
		if item.Value == category {
			return true
		}
	}
	return false
}

func parseUUIDList(values []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(values))
	for _, raw := range values {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func writeSiteMessageSSEHeaders(c *gin.Context) (http.Flusher, bool) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.InternalError(c, "SSE 不支持")
		return nil, false
	}
	return flusher, true
}

func siteMessageEventData(eventType string, unreadCount int64) string {
	if eventType == "init" {
		return fmt.Sprintf(`{"type":"init","unread_count":%d}`, unreadCount)
	}
	return fmt.Sprintf(`{"type":"%s"}`, eventType)
}
