package playbook

import (
	"testing"

	"github.com/company/auto-healing/internal/modules/integrations/model"
)

func TestValidatePlaybookVariablesAcceptsValidVariables(t *testing.T) {
	variables := model.JSONArray{
		map[string]interface{}{
			"name":        "log_path",
			"type":        "string",
			"required":    true,
			"default":     "/var/log/app.log",
			"description": "日志路径",
			"pattern":     `^/`,
		},
		map[string]interface{}{
			"name":    "cleanup_mode",
			"type":    "select",
			"enum":    []interface{}{"dry_run", "delete"},
			"default": "dry_run",
		},
		map[string]interface{}{
			"name":    "max_files",
			"type":    "integer",
			"default": float64(10),
			"min":     float64(1),
			"max":     float64(100),
		},
		map[string]interface{}{
			"name":    "extra_args",
			"type":    "list",
			"default": []interface{}{"--verbose"},
		},
	}

	if err := validatePlaybookVariables(variables); err != nil {
		t.Fatalf("validatePlaybookVariables() error = %v", err)
	}
}

func TestValidatePlaybookVariablesRejectsInvalidName(t *testing.T) {
	err := validatePlaybookVariables(model.JSONArray{
		map[string]interface{}{"name": "bad-name", "type": "string"},
	})
	if err == nil {
		t.Fatal("validatePlaybookVariables() error = nil, want invalid name rejection")
	}
}

func TestValidatePlaybookVariablesRejectsDuplicateName(t *testing.T) {
	err := validatePlaybookVariables(model.JSONArray{
		map[string]interface{}{"name": "target_host", "type": "string"},
		map[string]interface{}{"name": "target_host", "type": "string"},
	})
	if err == nil {
		t.Fatal("validatePlaybookVariables() error = nil, want duplicate name rejection")
	}
}

func TestValidatePlaybookVariablesRejectsEnumDefaultOutsideOptions(t *testing.T) {
	err := validatePlaybookVariables(model.JSONArray{
		map[string]interface{}{
			"name":    "mode",
			"type":    "select",
			"enum":    []interface{}{"safe", "force"},
			"default": "unknown",
		},
	})
	if err == nil {
		t.Fatal("validatePlaybookVariables() error = nil, want enum default rejection")
	}
}

func TestValidatePlaybookVariablesRejectsWrongDefaultType(t *testing.T) {
	err := validatePlaybookVariables(model.JSONArray{
		map[string]interface{}{"name": "retry_count", "type": "integer", "default": "3"},
	})
	if err == nil {
		t.Fatal("validatePlaybookVariables() error = nil, want wrong default type rejection")
	}
}
