package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/gin-gonic/gin"
)

func TestListAuditLogsRejectsInvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/audit", NewAuditHandlerWithDeps(AuditHandlerDeps{
		Repo: auditrepo.NewAuditLogRepository(nil),
	}).ListAuditLogs)

	req := httptest.NewRequest(http.MethodGet, "/audit?user_id=bad", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestListAuditLogsRejectsInvalidCreatedAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/audit", NewAuditHandlerWithDeps(AuditHandlerDeps{
		Repo: auditrepo.NewAuditLogRepository(nil),
	}).ListAuditLogs)

	req := httptest.NewRequest(http.MethodGet, "/audit?created_after=bad-time", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
