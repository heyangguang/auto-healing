package plugin

import (
	"testing"
	"time"
)

func TestCalculateNextSyncAtFromUsesBaseTime(t *testing.T) {
	base := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	next := calculateNextSyncAtFrom(base, true, 5)
	if next == nil {
		t.Fatal("expected next sync time")
	}
	expected := base.Add(5 * time.Minute)
	if !next.Equal(expected) {
		t.Fatalf("next = %v, want %v", *next, expected)
	}
}
