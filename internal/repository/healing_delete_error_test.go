package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestHealingFlowDeleteReturnsDatabaseErrorBeforeNotFound(t *testing.T) {
	db := newStateTestDB(t)
	repo := &HealingFlowRepository{db: db}
	ctx := WithTenantID(context.Background(), uuid.New())

	err := repo.Delete(ctx, uuid.New())
	if err == nil {
		t.Fatal("Delete() should return database error when table is missing")
	}
	if err == ErrHealingFlowNotFound {
		t.Fatal("Delete() should not mask database error as ErrHealingFlowNotFound")
	}
}

func TestHealingRuleDeleteReturnsDatabaseErrorBeforeNotFound(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			rule_id TEXT
		);
	`)

	repo := &HealingRuleRepository{db: db}
	ctx := WithTenantID(context.Background(), uuid.New())

	err := repo.Delete(ctx, uuid.New(), false)
	if err == nil {
		t.Fatal("Delete() should return database error when healing_rules table is missing")
	}
	if err == ErrHealingRuleNotFound {
		t.Fatal("Delete() should not mask database error as ErrHealingRuleNotFound")
	}
}
