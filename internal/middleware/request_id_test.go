package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDUsesIncomingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/id", nil)
	req.Header.Set(RequestIDKey, "req-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if got := recorder.Header().Get(RequestIDKey); got != "req-123" {
		t.Fatalf("response request id = %q, want req-123", got)
	}
}

func TestGetRequestIDReturnsEmptyForUnexpectedType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(RequestIDKey, 42)

	if got := GetRequestID(c); got != "" {
		t.Fatalf("GetRequestID() = %q, want empty", got)
	}
}
