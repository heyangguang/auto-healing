package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
	router.GET("/rules/:id", NewCommandBlacklistHandlerWithDeps(CommandBlacklistHandlerDeps{
		Service: opsservice.NewCommandBlacklistServiceWithDeps(opsservice.CommandBlacklistServiceDeps{
			Repo: opsrepo.NewCommandBlacklistRepositoryWithDB(&gorm.DB{}),
		}),
	}).Get)

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
