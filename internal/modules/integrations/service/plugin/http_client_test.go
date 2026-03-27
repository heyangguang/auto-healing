package plugin

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
)

func TestBuildRequestURLURLencodesParams(t *testing.T) {
	client := NewHTTPClient()
	built := client.buildRequestURL("https://example.com/api", model.JSON{
		"since_param": "updated_after",
		"extra_params": map[string]interface{}{
			"filter": "a&b=c",
		},
	}, time.Date(2026, 3, 26, 10, 0, 0, 0, time.FixedZone("CST", 8*3600)))

	if !strings.Contains(built, "filter=a%26b%3Dc") {
		t.Fatalf("expected extra params to be url-encoded, got %q", built)
	}
	if !strings.Contains(built, "updated_after=") || strings.Contains(built, "+08:00") {
		t.Fatalf("expected since param to be encoded, got %q", built)
	}
}

func TestAddAuthRejectsUnknownAuthType(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if err := NewHTTPClient().addAuth(req, model.JSON{"auth_type": "unknown"}); err == nil {
		t.Fatal("expected unknown auth_type to be rejected")
	}
}
