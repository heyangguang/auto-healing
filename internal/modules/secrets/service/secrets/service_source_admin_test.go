package secrets

import (
	"errors"
	"testing"

	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
)

func TestApplySourceAdminChangesRejectsNegativePriority(t *testing.T) {
	source := &secretsmodel.SecretsSource{}
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
	source := &secretsmodel.SecretsSource{}

	_, err := applySourceAdminChanges(source, nil, nil, "enabled")
	if err == nil {
		t.Fatal("applySourceAdminChanges() error = nil, want invalid input")
	}
	if !errors.Is(err, ErrSecretsSourceInvalidInput) {
		t.Fatalf("applySourceAdminChanges() error = %v, want invalid input", err)
	}
}

func TestApplySourceAdminChangesReturnsRequestedDefaultAndClearsStoredDefault(t *testing.T) {
	source := &secretsmodel.SecretsSource{
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
	source := &secretsmodel.SecretsSource{
		IsDefault: true,
		Priority:  2,
		Status:    "active",
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

func TestApplySourceAdminChangesRejectsInactiveDefault(t *testing.T) {
	source := &secretsmodel.SecretsSource{Status: "active"}
	inactive := "inactive"
	setDefault := true

	_, err := applySourceAdminChanges(source, &setDefault, nil, inactive)
	if err == nil {
		t.Fatalf("applySourceAdminChanges() expected error")
	}
	if source.Status != inactive {
		t.Fatalf("source.Status = %q, want %q", source.Status, inactive)
	}
}
