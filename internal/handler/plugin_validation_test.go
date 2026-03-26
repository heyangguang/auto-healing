package handler

import (
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestValidatePluginCreateRequestRejectsInvalidSyncInterval(t *testing.T) {
	req := &CreatePluginRequest{
		Type:                "itsm",
		SyncEnabled:         true,
		SyncIntervalMinutes: 0,
	}

	if err := validatePluginCreateRequest(req); err == nil {
		t.Fatal("expected invalid sync interval to be rejected")
	}
}

func TestValidatePluginUpdateRequestUsesCurrentState(t *testing.T) {
	current := &model.Plugin{SyncEnabled: true, SyncIntervalMinutes: 5}
	interval := 0

	if err := validatePluginUpdateRequest(current, &UpdatePluginRequest{SyncIntervalMinutes: &interval}); err == nil {
		t.Fatal("expected update to reject interval that breaks enabled sync")
	}
}

func TestValidatePluginCreateRequestRejectsUnknownType(t *testing.T) {
	if err := validatePluginCreateRequest(&CreatePluginRequest{Type: "unknown"}); err == nil {
		t.Fatal("expected unsupported plugin type to be rejected")
	}
}

func TestValidateNonNegativeMaxFailuresRejectsNegativeValue(t *testing.T) {
	value := -1
	if err := validateNonNegativeMaxFailures(&value); err == nil {
		t.Fatal("expected negative max_failures to be rejected")
	}
}
