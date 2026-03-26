package secrets

import (
	"errors"
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestApplySourceAdminChangesRejectsNegativePriority(t *testing.T) {
	source := &model.SecretsSource{}
	priority := -1

	_, err := applySourceAdminChanges(source, nil, &priority, "")
	if err == nil {
		t.Fatal("applySourceAdminChanges() error = nil, want invalid input")
	}
	if !errors.Is(err, ErrSecretsSourceInvalidInput) {
		t.Fatalf("applySourceAdminChanges() error = %v, want invalid input", err)
	}
}

func TestApplySourceAdminChangesRejectsInvalidStatus(t *testing.T) {
	source := &model.SecretsSource{}

	_, err := applySourceAdminChanges(source, nil, nil, "banana")
	if err == nil {
		t.Fatal("applySourceAdminChanges() error = nil, want invalid input")
	}
	if !errors.Is(err, ErrSecretsSourceInvalidInput) {
		t.Fatalf("applySourceAdminChanges() error = %v, want invalid input", err)
	}
}
