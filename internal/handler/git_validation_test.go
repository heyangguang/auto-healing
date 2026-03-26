package handler

import (
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestValidateGitCreateRequestRejectsInvalidSyncInterval(t *testing.T) {
	req := &CreateRepoRequest{
		SyncEnabled:  true,
		SyncInterval: "bad",
	}

	if err := validateGitCreateRequest(req); err == nil {
		t.Fatal("expected invalid sync interval to be rejected")
	}
}

func TestValidateGitCreateRequestAllowsDefaultSyncInterval(t *testing.T) {
	req := &CreateRepoRequest{
		SyncEnabled: true,
	}

	if err := validateGitCreateRequest(req); err != nil {
		t.Fatalf("expected default sync interval to be accepted, got %v", err)
	}
}

func TestValidateGitUpdateRequestRejectsNegativeMaxFailures(t *testing.T) {
	current := &model.GitRepository{SyncEnabled: false}
	value := -1

	if err := validateGitUpdateRequest(current, &UpdateRepoRequest{MaxFailures: &value}); err == nil {
		t.Fatal("expected negative max_failures to be rejected")
	}
}

func TestValidateGitAuthTypeRejectsUnknownType(t *testing.T) {
	if err := validateGitAuthType("invalid"); err == nil {
		t.Fatal("expected unsupported auth_type to be rejected")
	}
}
