package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/logger"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const maxAuditRequestBodyBytes = 10 * 1024

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func prepareAuditRequest(c *gin.Context, db *gorm.DB) (*auditRequestState, bool) {
	method := c.Request.Method
	path := c.Request.URL.Path
	if !shouldAuditRequest(method, path) {
		return nil, false
	}
	tenantID := uuid.Nil
	if resolvedTenantID, ok := accessrepo.TenantIDFromContextOK(c.Request.Context()); ok {
		tenantID = resolvedTenantID
	}
	state := &auditRequestState{
		method:          method,
		path:            path,
		requestBody:     readAuditRequestBody(c),
		startTime:       time.Now(),
		resourceID:      resolveAuditResourceID(c, path),
		resourceKey:     extractResourceKey(path),
		tenantID:        tenantID,
		isPlatformAdmin: IsPlatformAdmin(c),
	}
	state.oldState = loadAuditOldState(db, state)
	state.bodyWriter = &responseBodyWriter{ResponseWriter: c.Writer, body: bytes.NewBufferString("")}
	return state, true
}

func shouldAuditRequest(method, path string) bool {
	if method == "GET" || method == "OPTIONS" || method == "HEAD" {
		return false
	}
	return !shouldSkipAudit(path)
}

func readAuditRequestBody(c *gin.Context) []byte {
	if c.Request.Body == nil {
		return nil
	}

	probe, err := io.ReadAll(io.LimitReader(c.Request.Body, maxAuditRequestBodyBytes+1))
	c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(probe), c.Request.Body))
	if err != nil {
		logger.API("AUDIT").Warn("读取审计请求体失败: %v", err)
		return nil
	}
	if len(probe) > maxAuditRequestBodyBytes {
		return probe[:maxAuditRequestBodyBytes]
	}
	return probe
}

func resolveAuditResourceID(c *gin.Context, path string) *uuid.UUID {
	resourceID := extractResourceID(path)
	if !strings.HasSuffix(path, "/auth/profile") {
		return resourceID
	}
	userIDStr := GetUserID(c)
	if userIDStr == "" {
		return resourceID
	}
	if uid, err := uuid.Parse(userIDStr); err == nil {
		return &uid
	}
	return resourceID
}

func loadAuditOldState(db *gorm.DB, state *auditRequestState) map[string]interface{} {
	if state.method != "PUT" && state.method != "PATCH" && state.method != "DELETE" {
		return nil
	}
	tenantID := state.tenantID
	if isPlatformRoute(state.path) {
		tenantID = uuid.Nil
	}
	return captureOldState(db, state.path, state.resourceID, state.resourceKey, tenantID)
}

func captureAuditActor(c *gin.Context, bodyWriter *responseBodyWriter) auditActor {
	return auditActor{
		userID:          parseAuditUserID(GetUserID(c)),
		username:        GetUsername(c),
		statusCode:      c.Writer.Status(),
		ipAddress:       NormalizeIP(c.ClientIP()),
		userAgent:       c.Request.UserAgent(),
		responseBody:    bodyWriter.body.Bytes(),
		isImpersonating: IsImpersonating(c),
	}
}

func parseAuditUserID(userID string) *uuid.UUID {
	if userID == "" {
		return nil
	}
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return nil
	}
	return &parsed
}

func writeAuditLogs(state *auditRequestState, actor auditActor, repo *auditrepo.AuditLogRepository, platformRepo *auditrepo.PlatformAuditLogRepository, db *gorm.DB) {
	ensureMiddlewareLifecycle().Go(func(rootCtx context.Context) {
		defer recoverAuditPanic()
		event := buildAuditEvent(state, actor, db)
		persistAuditEvent(rootCtx, event, repo, platformRepo)
	})
}

func recoverAuditPanic() {
	if r := recover(); r != nil {
		logger.API("AUDIT").Error("审计日志记录失败 (panic): %v", r)
	}
}

func buildAuditEvent(state *auditRequestState, actor auditActor, db *gorm.DB) auditEvent {
	action, resourceType := inferActionAndResource(state.method, state.path)
	bodyJSON := parseAuditRequestBody(state.requestBody)
	resourceName, resourceErr := resolveResourceName(db, state.path, state.resourceID, state.resourceKey, bodyJSON, auditResourceTenantID(state.path, state.tenantID))
	if resourceErr != nil {
		logger.API("AUDIT").Error("解析审计资源名失败: path=%s resource_type=%s resource_id=%v resource_key=%s err=%v", state.path, resourceType, state.resourceID, state.resourceKey, resourceErr)
	}
	status, errorMessage := auditStatus(actor.statusCode, actor.responseBody)
	return auditEvent{
		userID:          actor.userID,
		username:        actor.username,
		method:          state.method,
		path:            state.path,
		resourceType:    resourceType,
		action:          action,
		resourceID:      state.resourceID,
		resourceName:    resourceName,
		bodyJSON:        bodyJSON,
		changes:         computeAuditChanges(status, state.method, action, state.oldState, bodyJSON),
		status:          status,
		errorMessage:    errorMessage,
		statusCode:      actor.statusCode,
		ipAddress:       actor.ipAddress,
		userAgent:       actor.userAgent,
		createdAt:       state.startTime,
		tenantID:        state.tenantID,
		isPlatformAdmin: state.isPlatformAdmin,
		isImpersonating: actor.isImpersonating,
	}
}

func parseAuditRequestBody(requestBody []byte) model.JSON {
	if len(requestBody) == 0 || len(requestBody) > maxAuditRequestBodyBytes {
		return nil
	}
	var bodyJSON model.JSON
	if json.Unmarshal(requestBody, &bodyJSON) != nil {
		return nil
	}
	return sanitizeAuditJSON(bodyJSON)
}

func auditResourceTenantID(path string, tenantID uuid.UUID) uuid.UUID {
	if isPlatformRoute(path) {
		return uuid.Nil
	}
	return tenantID
}

func auditStatus(statusCode int, responseBody []byte) (string, string) {
	if statusCode >= 400 {
		return "failed", extractErrorMessage(responseBody)
	}
	return "success", ""
}

func computeAuditChanges(status, method, action string, oldState map[string]interface{}, bodyJSON model.JSON) model.JSON {
	if status != "success" {
		return nil
	}
	return computeChanges(method, action, sanitizeAuditMap(oldState), bodyJSON)
}

func persistAuditEvent(rootCtx context.Context, event auditEvent, repo *auditrepo.AuditLogRepository, platformRepo *auditrepo.PlatformAuditLogRepository) {
	ctx, cancel := newAuditContext(rootCtx, event.tenantID)
	defer cancel()
	if event.isPlatformAdmin {
		persistPlatformAuditEvent(ctx, event, repo, platformRepo)
		return
	}
	if err := repo.Create(ctx, newTenantAuditLog(event, event.username)); err != nil {
		logger.API("AUDIT").Error("审计日志写入失败: %v", err)
	}
}

func newAuditContext(rootCtx context.Context, tenantID uuid.UUID) (context.Context, context.CancelFunc) {
	baseCtx := auditrepo.WithTenantID(context.WithoutCancel(rootCtx), tenantID)
	return context.WithTimeout(baseCtx, 5*time.Second)
}

func persistPlatformAuditEvent(ctx context.Context, event auditEvent, repo *auditrepo.AuditLogRepository, platformRepo *auditrepo.PlatformAuditLogRepository) {
	if err := platformRepo.Create(ctx, newPlatformAuditLog(event)); err != nil {
		logger.API("AUDIT").Error("平台审计日志写入失败: %v", err)
	}
	if event.isImpersonating {
		if err := repo.Create(ctx, newTenantAuditLog(event, event.username+" [Impersonation]")); err != nil {
			logger.API("AUDIT").Error("Impersonation 租户审计日志写入失败: %v", err)
		}
	}
}
