package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"

	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/gin-gonic/gin"
)

func TestListPlatformAuditLogsRejectsInvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/platform/audit", NewPlatformAuditHandlerWithDeps(PlatformAuditHandlerDeps{
		Repo: auditrepo.NewPlatformAuditLogRepositoryWithDB(&gorm.DB{}),
	}).ListPlatformAuditLogs)

	req := httptest.NewRequest(http.MethodGet, "/platform/audit?user_id=bad", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestListPlatformAuditLogsRejectsInvalidCreatedBefore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/platform/audit", NewPlatformAuditHandlerWithDeps(PlatformAuditHandlerDeps{
		Repo: auditrepo.NewPlatformAuditLogRepositoryWithDB(&gorm.DB{}),
	}).ListPlatformAuditLogs)

	req := httptest.NewRequest(http.MethodGet, "/platform/audit?created_before=bad-time", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
