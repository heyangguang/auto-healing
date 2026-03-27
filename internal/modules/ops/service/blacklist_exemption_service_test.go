package service

import (
	"errors"
	"testing"

	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
)

func TestNormalizeBlacklistExemptionMutationErrorPreservesSentinel(t *testing.T) {
	err := normalizeBlacklistExemptionMutationError(opsrepo.ErrBlacklistExemptionNotPending)
	if !errors.Is(err, opsrepo.ErrBlacklistExemptionNotPending) {
		t.Fatalf("error = %v, want ErrBlacklistExemptionNotPending", err)
	}
}
