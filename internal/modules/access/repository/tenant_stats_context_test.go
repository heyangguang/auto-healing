package repository

import (
	"context"
	"errors"
	"testing"
)

func TestTenantRepositoryGetTrendByDayRejectsInvalidTable(t *testing.T) {
	db := newStateTestDB(t)
	repo := NewTenantRepositoryWithDB(db)

	_, _, err := repo.GetTrendByDay(context.Background(), "not_allowed_table", 7)
	if !errors.Is(err, ErrTenantStatsTableNotAllowed) {
		t.Fatalf("GetTrendByDay() error = %v, want %v", err, ErrTenantStatsTableNotAllowed)
	}
}

func TestTenantRepositoryGetTrendByDayWhereRejectsInvalidTable(t *testing.T) {
	db := newStateTestDB(t)
	repo := NewTenantRepositoryWithDB(db)

	_, _, err := repo.GetTrendByDayWhere(context.Background(), "not_allowed_table", 7, "")
	if !errors.Is(err, ErrTenantStatsTableNotAllowed) {
		t.Fatalf("GetTrendByDayWhere() error = %v, want %v", err, ErrTenantStatsTableNotAllowed)
	}
}
