package playbook

import "testing"

func TestApplyPlaybookUpdatePreservesOmittedFields(t *testing.T) {
	name := "origin"
	description := "origin-desc"
	newDescription := "new-desc"

	applyPlaybookUpdate(&name, &description, &UpdateInput{
		Description: &newDescription,
	})

	if name != "origin" {
		t.Fatalf("name = %q, want origin", name)
	}
	if description != newDescription {
		t.Fatalf("description = %q, want %q", description, newDescription)
	}
}

func TestValidatePlaybookUpdateInputRejectsEmptyName(t *testing.T) {
	name := "   "
	if err := validatePlaybookUpdateInput(&UpdateInput{Name: &name}); err == nil {
		t.Fatal("expected empty playbook name to be rejected")
	}
}
