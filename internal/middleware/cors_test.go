package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSOptionsIncludesTenantAndImpersonationHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORS())
	router.OPTIONS("/resource", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	allowHeaders := recorder.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{"X-Request-ID", "X-Tenant-ID", "X-Impersonation", "X-Impersonation-Request-ID"} {
		if !strings.Contains(allowHeaders, header) {
			t.Fatalf("allow headers %q missing %q", allowHeaders, header)
		}
	}
	exposeHeaders := recorder.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposeHeaders, "X-Refresh-Token") {
		t.Fatalf("expose headers %q missing X-Refresh-Token", exposeHeaders)
	}
	vary := strings.Join(recorder.Header().Values("Vary"), ",")
	for _, key := range []string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"} {
		if !strings.Contains(vary, key) {
			t.Fatalf("vary %q missing %q", vary, key)
		}
	}
}
