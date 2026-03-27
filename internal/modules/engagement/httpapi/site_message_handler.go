package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/modules/engagement/model"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	platformevents "github.com/company/auto-healing/internal/platform/events"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SiteMessageHandler 站内信处理器
type SiteMessageHandler struct {
	repo             *engagementrepo.SiteMessageRepository
	dictionaryRepo   *opsrepo.DictionaryRepository
	platformSettings *settingsrepo.PlatformSettingsRepository
	eventBus         *platformevents.MessageEventBus
	tenantRepo       *accessrepo.TenantRepository
	userRepo         *accessrepo.UserRepository
}

type SiteMessageHandlerDeps struct {
	SiteMessageRepo      *engagementrepo.SiteMessageRepository
	DictionaryRepo       *opsrepo.DictionaryRepository
	PlatformSettingsRepo *settingsrepo.PlatformSettingsRepository
	EventBus             *platformevents.MessageEventBus
	TenantRepo           *accessrepo.TenantRepository
	UserRepo             *accessrepo.UserRepository
}

func NewSiteMessageHandlerWithDeps(deps SiteMessageHandlerDeps) *SiteMessageHandler {
	requireSiteMessageHandlerDeps(deps)
	return &SiteMessageHandler{
		repo:             deps.SiteMessageRepo,
		dictionaryRepo:   deps.DictionaryRepo,
		platformSettings: deps.PlatformSettingsRepo,
		eventBus:         deps.EventBus,
		tenantRepo:       deps.TenantRepo,
		userRepo:         deps.UserRepo,
	}
}

func requireSiteMessageHandlerDeps(deps SiteMessageHandlerDeps) {
	switch {
	case deps.SiteMessageRepo == nil:
		panic("site message handler requires site message repository")
	case deps.DictionaryRepo == nil:
		panic("site message handler requires dictionary repository")
	case deps.PlatformSettingsRepo == nil:
		panic("site message handler requires platform settings repository")
	case deps.EventBus == nil:
		panic("site message handler requires message event bus")
	case deps.TenantRepo == nil:
		panic("site message handler requires tenant repository")
	case deps.UserRepo == nil:
		panic("site message handler requires user repository")
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

func (h *SiteMessageHandler) isValidSiteMessageCategory(ctx context.Context, category string) (bool, error) {
	items, err := h.siteMessageCategoryItems(ctx)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item.Value == category {
			return true, nil
		}
	}
	return false, nil
}

func (h *SiteMessageHandler) siteMessageCategoryItems(ctx context.Context) ([]model.SiteMessageCategoryInfo, error) {
	items, err := h.dictionaryRepo.ListByTypes(ctx, []string{"site_message_category"}, true)
	if err != nil {
		return nil, err
	}
	result := make([]model.SiteMessageCategoryInfo, 0, len(items))
	for _, item := range items {
		result = append(result, model.SiteMessageCategoryInfo{
			Value: item.DictKey,
			Label: item.Label,
		})
	}
	return result, nil
}

func (h *SiteMessageHandler) getSiteMessageSettings(ctx context.Context) (siteMessageSettingsResponse, error) {
	setting, err := h.platformSettings.GetByKey(ctx, "site_message.retention_days")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return siteMessageSettingsResponse{RetentionDays: 90}, nil
		}
		return siteMessageSettingsResponse{}, err
	}
	retentionDays, err := strconv.Atoi(setting.Value)
	if err != nil {
		return siteMessageSettingsResponse{}, err
	}
	if retentionDays < 1 {
		return siteMessageSettingsResponse{}, fmt.Errorf("invalid site_message.retention_days: %d", retentionDays)
	}
	return siteMessageSettingsResponse{
		RetentionDays: retentionDays,
		UpdatedAt:     setting.UpdatedAt.Format("2006-01-02T15:04:05.000000-07:00"),
	}, nil
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
