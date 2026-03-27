package repository

import "testing"

func TestScheduleOrderClauseAcceptsUppercaseDesc(t *testing.T) {
	clause := scheduleOrderClause(&ScheduleListOptions{
		SortBy:    "name",
		SortOrder: "DESC",
	})
	if clause != "name DESC" {
		t.Fatalf("clause = %q, want %q", clause, "name DESC")
	}
}
