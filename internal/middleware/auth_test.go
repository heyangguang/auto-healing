package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAllowQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "site message events", path: "/api/v1/tenant/site-messages/events", want: true},
		{name: "execution stream", path: "/api/v1/tenant/execution-runs/123/stream", want: true},
		{name: "healing events", path: "/api/v1/tenant/healing/instances/123/events", want: true},
		{name: "normal api", path: "/api/v1/common/search", want: false},
		{name: "auth refresh", path: "/api/v1/auth/refresh", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest("GET", "http://example.com"+tt.path+"?token=abc", nil)
			c.Request = req

			if got := allowQueryToken(c); got != tt.want {
				t.Fatalf("allowQueryToken(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
