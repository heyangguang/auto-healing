package secrets

import (
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestApplySourceAdminChangesRejectsNegativePriority(t *testing.T) {
	source := &model.SecretsSource{}
	priority := -1

	_, err := applySourceAdminChanges(source, nil, &priority, "")

	if err == nil {
		t.Fatal("applySourceAdminChanges() error = nil, want negative priority error")
	}
}

func TestApplySourceAdminChangesRejectsInvalidStatus(t *testing.T) {
	source := &model.SecretsSource{}

	_, err := applySourceAdminChanges(source, nil, nil, "enabled")

	if err == nil {
		t.Fatal("applySourceAdminChanges() error = nil, want invalid status error")
	}
}

func TestApplySourceAdminChangesReturnsRequestedDefaultAndClearsStoredDefault(t *testing.T) {
	source := &model.SecretsSource{
		IsDefault: false,
		Priority:  1,
		Status:    "inactive",
	}
	isDefault := true
	priority := 3

	requestedDefault, err := applySourceAdminChanges(source, &isDefault, &priority, "active")

	if err != nil {
		t.Fatalf("applySourceAdminChanges() error = %v", err)
	}
	if !requestedDefault {
		t.Fatal("requestedDefault = false, want true")
	}
	if source.IsDefault {
		t.Fatal("source.IsDefault = true, want false before repo.SetDefault")
	}
	if source.Priority != 3 {
		t.Fatalf("source.Priority = %d, want 3", source.Priority)
	}
	if source.Status != "active" {
		t.Fatalf("source.Status = %q, want active", source.Status)
	}
}

func TestApplySourceAdminChangesLeavesDefaultUnchangedWhenUnset(t *testing.T) {
	source := &model.SecretsSource{
		IsDefault: true,
		Priority:  2,
		Status:    "inactive",
	}

	requestedDefault, err := applySourceAdminChanges(source, nil, nil, "")

	if err != nil {
		t.Fatalf("applySourceAdminChanges() error = %v", err)
	}
	if requestedDefault {
		t.Fatal("requestedDefault = true, want false")
	}
	if !source.IsDefault {
		t.Fatal("source.IsDefault = false, want true")
	}
}
