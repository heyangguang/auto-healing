package service

import (
	"errors"
	"testing"

	"github.com/company/auto-healing/internal/repository"
)

func TestNormalizeBlacklistExemptionMutationErrorPreservesSentinel(t *testing.T) {
	err := normalizeBlacklistExemptionMutationError(repository.ErrBlacklistExemptionNotPending)
	if !errors.Is(err, repository.ErrBlacklistExemptionNotPending) {
		t.Fatalf("error = %v, want ErrBlacklistExemptionNotPending", err)
	}
}
