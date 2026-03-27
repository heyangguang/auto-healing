package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListPlatformAuditLogsRejectsInvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/platform/audit", NewPlatformAuditHandler().ListPlatformAuditLogs)

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
	router.GET("/platform/audit", NewPlatformAuditHandler().ListPlatformAuditLogs)

	req := httptest.NewRequest(http.MethodGet, "/platform/audit?created_before=bad-time", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
