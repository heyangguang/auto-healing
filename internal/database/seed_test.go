package database

import "testing"

func TestAllPermissionsIncludeSensitiveCapabilityPermissions(t *testing.T) {
	if !permissionExists("secrets:query") {
		t.Fatal("missing permission seed: secrets:query")
	}
	if !permissionExists("repository:validate") {
		t.Fatal("missing permission seed: repository:validate")
	}
}

func TestSystemRolesAssignSensitiveCapabilityPermissions(t *testing.T) {
	assertRoleHasPermission(t, "admin", "secrets:query")
	assertRoleHasPermission(t, "admin", "repository:validate")
	assertRoleHasPermission(t, "impersonation_accessor", "secrets:query")
	assertRoleHasPermission(t, "impersonation_accessor", "repository:validate")
	assertRoleHasPermission(t, "operator", "secrets:query")
	assertRoleHasPermission(t, "devops_engineer", "secrets:query")
	assertRoleHasPermission(t, "devops_engineer", "repository:validate")

	assertRoleLacksPermission(t, "viewer", "secrets:query")
	assertRoleLacksPermission(t, "auditor", "repository:validate")
	assertRoleLacksPermission(t, "monitor_admin", "secrets:query")
}

func permissionExists(code string) bool {
	for _, permission := range AllPermissions {
		if permission.Code == code {
			return true
		}
	}
	return false
}

func assertRoleHasPermission(t *testing.T, roleName, permissionCode string) {
	t.Helper()
	role := findRole(roleName)
	if role == nil {
		t.Fatalf("role %q not found", roleName)
	}
	for _, permission := range role.Permissions {
		if permission == permissionCode {
			return
		}
	}
	t.Fatalf("role %q missing permission %q", roleName, permissionCode)
}

func assertRoleLacksPermission(t *testing.T, roleName, permissionCode string) {
	t.Helper()
	role := findRole(roleName)
	if role == nil {
		t.Fatalf("role %q not found", roleName)
	}
	for _, permission := range role.Permissions {
		if permission == permissionCode {
			t.Fatalf("role %q should not include permission %q", roleName, permissionCode)
		}
	}
}

func findRole(name string) *RoleSeed {
	for i := range SystemRoles {
		if SystemRoles[i].Name == name {
			return &SystemRoles[i]
		}
	}
	return nil
}
