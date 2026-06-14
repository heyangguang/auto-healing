package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBuildRunListOptionsDoesNotDefaultTriggeredByFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/runs", nil)

	opts := buildRunListOptions(ctx, 1, 20)

	if opts.TriggeredBy != "" {
		t.Fatalf("TriggeredBy = %q, want empty filter", opts.TriggeredBy)
	}
}

func TestBuildRunListOptionsNormalizesExplicitTriggeredByFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/runs?triggered_by=healing:auto", nil)

	opts := buildRunListOptions(ctx, 1, 20)

	if opts.TriggeredBy != "healing" {
		t.Fatalf("TriggeredBy = %q, want healing", opts.TriggeredBy)
	}
}
