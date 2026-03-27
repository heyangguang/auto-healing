package httpapi

import (
	"testing"

	"github.com/google/uuid"
)

func TestResolveCreateUserRoleIDsRejectsMixedRoleSelectors(t *testing.T) {
	h := &UserHandler{}
	roleID := uuid.New()

	_, err := h.resolveCreateUserRoleIDs(nil, &roleID, []uuid.UUID{uuid.New()})
	if err == nil {
		t.Fatalf("resolveCreateUserRoleIDs() error = nil, want rejection")
	}
	if err.Error() != "role_id 和 role_ids 不能同时传递" {
		t.Fatalf("resolveCreateUserRoleIDs() error = %q", err.Error())
	}
}

func TestDedupeRoleIDsPreservesOrder(t *testing.T) {
	roleA := uuid.New()
	roleB := uuid.New()

	deduped := dedupeRoleIDs([]uuid.UUID{roleA, roleB, roleA, roleB})
	if len(deduped) != 2 {
		t.Fatalf("dedupeRoleIDs() len = %d, want 2", len(deduped))
	}
	if deduped[0] != roleA || deduped[1] != roleB {
		t.Fatalf("dedupeRoleIDs() = %v, want [%s %s]", deduped, roleA, roleB)
	}
}
