package schedulerx

import (
	"testing"

	"github.com/google/uuid"
)

func TestInFlightSetRetainKeepsEntryUntilFinalFinish(t *testing.T) {
	set := NewInFlightSet()
	id := uuid.New()

	if !set.Start(id) {
		t.Fatal("expected initial start to succeed")
	}
	if !set.Retain(id) {
		t.Fatal("expected retain to succeed for active entry")
	}

	set.Finish(id)

	if set.Start(id) {
		t.Fatal("entry should still be retained after first finish")
	}

	set.Finish(id)

	if !set.Start(id) {
		t.Fatal("entry should be released after final finish")
	}
}
