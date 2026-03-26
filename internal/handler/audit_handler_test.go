package handler

import (
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestSanitizeAuditPayloadMasksNestedSensitiveFields(t *testing.T) {
	payload := model.JSON{
		"config": map[string]interface{}{
			"auth": map[string]interface{}{
				"token":       "secret-token",
				"password":    "secret-password",
				"private_key": "secret-key",
				"secret_id":   "secret-id",
			},
			"safe": "ok",
		},
	}

	masked := sanitizeAuditPayload(payload)
	config := masked["config"].(map[string]interface{})
	auth := config["auth"].(map[string]interface{})

	for _, key := range []string{"token", "password", "private_key", "secret_id"} {
		if auth[key] != "***" {
			t.Fatalf("%s = %#v, want masked", key, auth[key])
		}
	}
	if config["safe"] != "ok" {
		t.Fatalf("safe = %#v, want ok", config["safe"])
	}
}
