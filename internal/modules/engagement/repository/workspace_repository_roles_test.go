package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestWorkspaceRepositoryDeleteRejectsDefaultWorkspace(t *testing.T) {
	db := newSQLiteTestDB(t)
	createWorkspaceSchema(t, db)

	repo := NewWorkspaceRepositoryWithDB(db)
	tenantID := uuid.New()
	workspaceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO system_workspaces (id, tenant_id, name, description, config, is_default, created_at, updated_at) VALUES (?, ?, ?, '', '{}', ?, ?, ?)`,
		workspaceID.String(), tenantID.String(), "default", true, now, now)

	err := repo.Delete(platformrepo.WithTenantID(context.Background(), tenantID), workspaceID)
	if !errors.Is(err, ErrDefaultSystemWorkspaceProtected) {
		t.Fatalf("Delete() error = %v, want %v", err, ErrDefaultSystemWorkspaceProtected)
	}
}

func TestWorkspaceRepositoryAssignToRoleIgnoresDefaultWorkspaceIDs(t *testing.T) {
	db := newSQLiteTestDB(t)
	createWorkspaceSchema(t, db)

	repo := NewWorkspaceRepositoryWithDB(db)
	tenantID := uuid.New()
	roleID := uuid.New()
	defaultID := uuid.New()
	explicitID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO system_workspaces (id, tenant_id, name, description, config, is_default, created_at, updated_at) VALUES (?, ?, ?, '', '{}', ?, ?, ?)`,
		defaultID.String(), tenantID.String(), "default", true, now, now)
	mustExec(t, db, `INSERT INTO system_workspaces (id, tenant_id, name, description, config, is_default, created_at, updated_at) VALUES (?, ?, ?, '', '{}', ?, ?, ?)`,
		explicitID.String(), tenantID.String(), "explicit", false, now, now)

	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	if err := repo.AssignToRole(ctx, roleID, []uuid.UUID{defaultID, explicitID, defaultID}); err != nil {
		t.Fatalf("AssignToRole() error = %v", err)
	}

	explicitIDs, err := repo.GetRoleExplicitWorkspaceIDs(ctx, roleID)
	if err != nil {
		t.Fatalf("GetRoleExplicitWorkspaceIDs() error = %v", err)
	}
	if len(explicitIDs) != 1 || explicitIDs[0] != explicitID {
		t.Fatalf("explicit IDs = %#v, want only explicit workspace", explicitIDs)
	}

	allIDs, err := repo.GetRoleWorkspaceIDs(ctx, roleID)
	if err != nil {
		t.Fatalf("GetRoleWorkspaceIDs() error = %v", err)
	}
	got := map[uuid.UUID]bool{}
	for _, id := range allIDs {
		got[id] = true
	}
	if !got[defaultID] || !got[explicitID] || len(got) != 2 {
		t.Fatalf("workspace IDs = %#v, want default + explicit", allIDs)
	}
}

func TestWorkspaceRepositoryAssignToRoleRejectsOutOfTenantWorkspaceIDs(t *testing.T) {
	db := newSQLiteTestDB(t)
	createWorkspaceSchema(t, db)

	repo := NewWorkspaceRepositoryWithDB(db)
	tenantA := uuid.New()
	tenantB := uuid.New()
	roleID := uuid.New()
	otherTenantWorkspaceID := uuid.New()
	missingWorkspaceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO system_workspaces (id, tenant_id, name, description, config, is_default, created_at, updated_at) VALUES (?, ?, ?, '', '{}', ?, ?, ?)`,
		otherTenantWorkspaceID.String(), tenantB.String(), "other-tenant", false, now, now)

	err := repo.AssignToRole(
		platformrepo.WithTenantID(context.Background(), tenantA),
		roleID,
		[]uuid.UUID{otherTenantWorkspaceID, missingWorkspaceID},
	)

	var scopeErr *WorkspaceScopeError
	if !errors.As(err, &scopeErr) {
		t.Fatalf("AssignToRole() error = %v, want WorkspaceScopeError", err)
	}
	if len(scopeErr.IDs) != 2 {
		t.Fatalf("scope error IDs = %#v, want 2 rejected IDs", scopeErr.IDs)
	}
}
