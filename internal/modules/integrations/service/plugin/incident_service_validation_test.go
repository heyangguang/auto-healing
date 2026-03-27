package plugin

import (
	"errors"
	"testing"
)

func TestBatchResetScanRequiresScopeReturnsSentinel(t *testing.T) {
	svc := &IncidentService{}
	_, err := svc.BatchResetScan(nil, nil, "")
	if !errors.Is(err, ErrBatchResetScanScopeRequired) {
		t.Fatalf("err = %v, want %v", err, ErrBatchResetScanScopeRequired)
	}
}
