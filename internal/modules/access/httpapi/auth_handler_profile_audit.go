package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *AuthHandler) loadLoginHistoryItems(c *gin.Context, userID uuid.UUID, limit int) ([]LoginHistoryItem, error) {
	if middleware.IsPlatformAdmin(c) {
		logs, err := h.platformAuditRepo.GetUserLoginHistory(c.Request.Context(), userID, limit)
		if err != nil {
			return nil, err
		}
		return buildPlatformLoginHistoryItems(logs), nil
	}
	tenantID, err := h.authTenantIDOrError(c)
	if err != nil {
		return nil, err
	}
	logs, err := h.auditRepo.GetUserLoginHistory(c.Request.Context(), userID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	return buildTenantLoginHistoryItems(logs), nil
}

func (h *AuthHandler) loadProfileActivityItems(c *gin.Context, userID uuid.UUID, limit int) ([]ProfileActivityItem, error) {
	if middleware.IsPlatformAdmin(c) {
		logs, err := h.platformAuditRepo.GetUserActivities(c.Request.Context(), userID, limit)
		if err != nil {
			return nil, err
		}
		return buildPlatformActivityItems(logs), nil
	}
	tenantID, err := h.authTenantIDOrError(c)
	if err != nil {
		return nil, err
	}
	logs, err := h.auditRepo.GetUserActivities(c.Request.Context(), userID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	return buildTenantActivityItems(logs), nil
}

func buildPlatformLoginHistoryItems(logs []platformmodel.PlatformAuditLog) []LoginHistoryItem {
	items := make([]LoginHistoryItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, LoginHistoryItem{
			ID:           log.ID,
			Action:       log.Action,
			IPAddress:    log.IPAddress,
			UserAgent:    log.UserAgent,
			Status:       log.Status,
			ErrorMessage: sanitizeLoginHistoryErrorMessage(log.Status, log.ResponseStatus, log.ErrorMessage),
			CreatedAt:    log.CreatedAt,
		})
	}
	if items == nil {
		return []LoginHistoryItem{}
	}
	return items
}

func buildTenantLoginHistoryItems(logs []platformmodel.AuditLog) []LoginHistoryItem {
	items := make([]LoginHistoryItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, LoginHistoryItem{
			ID:           log.ID,
			Action:       log.Action,
			IPAddress:    log.IPAddress,
			UserAgent:    log.UserAgent,
			Status:       log.Status,
			ErrorMessage: sanitizeLoginHistoryErrorMessage(log.Status, log.ResponseStatus, log.ErrorMessage),
			CreatedAt:    log.CreatedAt,
		})
	}
	if items == nil {
		return []LoginHistoryItem{}
	}
	return items
}

func buildPlatformActivityItems(logs []platformmodel.PlatformAuditLog) []ProfileActivityItem {
	items := make([]ProfileActivityItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, ProfileActivityItem{
			ID:           log.ID,
			Action:       log.Action,
			ResourceType: log.ResourceType,
			ResourceName: log.ResourceName,
			Status:       log.Status,
			IPAddress:    log.IPAddress,
			CreatedAt:    log.CreatedAt,
		})
	}
	if items == nil {
		return []ProfileActivityItem{}
	}
	return items
}

func buildTenantActivityItems(logs []platformmodel.AuditLog) []ProfileActivityItem {
	items := make([]ProfileActivityItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, ProfileActivityItem{
			ID:           log.ID,
			Action:       log.Action,
			ResourceType: log.ResourceType,
			ResourceName: log.ResourceName,
			Status:       log.Status,
			IPAddress:    log.IPAddress,
			CreatedAt:    log.CreatedAt,
		})
	}
	if items == nil {
		return []ProfileActivityItem{}
	}
	return items
}
