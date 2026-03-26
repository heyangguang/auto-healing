package middleware

import (
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

type auditRequestState struct {
	method          string
	path            string
	requestBody     []byte
	startTime       time.Time
	resourceID      *uuid.UUID
	resourceKey     string
	tenantID        uuid.UUID
	isPlatformAdmin bool
	oldState        map[string]interface{}
	bodyWriter      *responseBodyWriter
}

type auditActor struct {
	userID          *uuid.UUID
	username        string
	statusCode      int
	ipAddress       string
	userAgent       string
	responseBody    []byte
	isImpersonating bool
}

type auditEvent struct {
	userID          *uuid.UUID
	username        string
	method          string
	path            string
	resourceType    string
	action          string
	resourceID      *uuid.UUID
	resourceName    string
	bodyJSON        model.JSON
	changes         model.JSON
	status          string
	errorMessage    string
	statusCode      int
	ipAddress       string
	userAgent       string
	createdAt       time.Time
	tenantID        uuid.UUID
	isPlatformAdmin bool
	isImpersonating bool
}

func newPlatformAuditLog(event auditEvent) *model.PlatformAuditLog {
	return &model.PlatformAuditLog{
		UserID:         event.userID,
		Username:       event.username,
		IPAddress:      event.ipAddress,
		UserAgent:      event.userAgent,
		Category:       "operation",
		Action:         event.action,
		ResourceType:   event.resourceType,
		ResourceID:     event.resourceID,
		ResourceName:   event.resourceName,
		RequestMethod:  event.method,
		RequestPath:    event.path,
		RequestBody:    event.bodyJSON,
		ResponseStatus: &event.statusCode,
		Changes:        event.changes,
		Status:         event.status,
		ErrorMessage:   event.errorMessage,
		CreatedAt:      event.createdAt,
	}
}

func newTenantAuditLog(event auditEvent, username string) *model.AuditLog {
	return &model.AuditLog{
		UserID:         event.userID,
		Username:       username,
		IPAddress:      event.ipAddress,
		UserAgent:      event.userAgent,
		Category:       "operation",
		Action:         event.action,
		ResourceType:   event.resourceType,
		ResourceID:     event.resourceID,
		ResourceName:   event.resourceName,
		RequestMethod:  event.method,
		RequestPath:    event.path,
		RequestBody:    event.bodyJSON,
		ResponseStatus: &event.statusCode,
		Changes:        event.changes,
		Status:         event.status,
		ErrorMessage:   event.errorMessage,
		CreatedAt:      event.createdAt,
	}
}
