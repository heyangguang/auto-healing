package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestCommandBlacklistListRejectsInvalidBoolQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/rules", (&CommandBlacklistHandler{}).List)

	req := httptest.NewRequest(http.MethodGet, "/rules?is_active=maybe", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestCommandBlacklistGetTreatsMissingTenantContextAsInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/rules/:id", NewCommandBlacklistHandler().Get)

	req := httptest.NewRequest(http.MethodGet, "/rules/"+uuid.NewString(), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestCommandBlacklistBatchToggleRequiresIsActive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/rules/batch-toggle", (&CommandBlacklistHandler{}).BatchToggle)

	req := httptest.NewRequest(http.MethodPost, "/rules/batch-toggle", strings.NewReader(`{"ids":["`+uuid.NewString()+`"]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
