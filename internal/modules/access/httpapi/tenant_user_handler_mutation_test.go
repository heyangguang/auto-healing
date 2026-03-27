package httpapi

import (
	"testing"

	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/google/uuid"
)

func TestValidateTenantScopedRegisterRequestRejectsRoleIDs(t *testing.T) {
	req := &authService.RegisterRequest{
		RoleIDs: []uuid.UUID{uuid.New()},
	}

	err := validateTenantScopedRegisterRequest(req)
	if err == nil {
		t.Fatalf("validateTenantScopedRegisterRequest() error = nil, want rejection")
	}
	if err.Error() != "租户侧创建用户不能直接分配平台角色，请创建后在当前租户内分配租户角色" {
		t.Fatalf("validateTenantScopedRegisterRequest() error = %q", err.Error())
	}
}

func TestValidateTenantScopedRegisterRequestAllowsEmptyRoleIDs(t *testing.T) {
	err := validateTenantScopedRegisterRequest(&authService.RegisterRequest{})
	if err != nil {
		t.Fatalf("validateTenantScopedRegisterRequest() error = %v", err)
	}
}

func TestHasTenantUserGlobalMutationRejectsGlobalFields(t *testing.T) {
	roleID := uuid.New()
	cases := []UpdateUserRequest{
		{DisplayName: "tenant-admin-edit", RoleID: &roleID},
		{Phone: "13800000000", RoleID: &roleID},
		{Status: "disabled", RoleID: &roleID},
	}

	for _, req := range cases {
		if !hasTenantUserGlobalMutation(req) {
			t.Fatalf("hasTenantUserGlobalMutation(%+v) = false, want true", req)
		}
	}
}

func TestHasTenantUserGlobalMutationAllowsRoleOnlyRequest(t *testing.T) {
	req := UpdateUserRequest{RoleID: ptrUUID(uuid.New())}
	if hasTenantUserGlobalMutation(req) {
		t.Fatalf("hasTenantUserGlobalMutation(%+v) = true, want false", req)
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}
