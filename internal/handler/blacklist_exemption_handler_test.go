package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestBlacklistExemptionRejectRejectsInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &BlacklistExemptionHandler{}
	router := gin.New()
	router.POST("/exemptions/:id/reject", h.Reject)

	req := httptest.NewRequest(http.MethodPost, "/exemptions/"+uuid.NewString()+"/reject", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestBlacklistExemptionApproveRejectsMissingUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &BlacklistExemptionHandler{}
	router := gin.New()
	router.POST("/exemptions/:id/approve", h.Approve)

	req := httptest.NewRequest(http.MethodPost, "/exemptions/"+uuid.NewString()+"/approve", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
