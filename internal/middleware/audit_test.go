package middleware

import (
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestSanitizeAuditJSONMasksNestedSensitiveFields(t *testing.T) {
	payload := model.JSON{
		"config": map[string]interface{}{
			"auth": map[string]interface{}{
				"token":       "secret-token",
				"password":    "secret-password",
				"private_key": "secret-key",
			},
			"nested_json": `{"secret_id":"abc","safe":"ok"}`,
		},
		"safe": "value",
	}

	masked := sanitizeAuditJSON(payload)

	config, ok := masked["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("config type = %T, want map[string]interface{}", masked["config"])
	}
	auth := config["auth"].(map[string]interface{})
	if auth["token"] != "***" || auth["password"] != "***" || auth["private_key"] != "***" {
		t.Fatalf("nested auth was not masked: %#v", auth)
	}

	nestedJSON, ok := config["nested_json"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested_json type = %T, want parsed masked map", config["nested_json"])
	}
	if nestedJSON["secret_id"] != "***" {
		t.Fatalf("secret_id = %#v, want masked", nestedJSON["secret_id"])
	}
	if nestedJSON["safe"] != "ok" {
		t.Fatalf("safe = %#v, want ok", nestedJSON["safe"])
	}
	if masked["safe"] != "value" {
		t.Fatalf("safe field changed unexpectedly: %#v", masked["safe"])
	}
}
