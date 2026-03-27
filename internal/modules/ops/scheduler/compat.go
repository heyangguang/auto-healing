package scheduler

import (
	"os"
	"time"

	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
)

func NewBlacklistExemptionScheduler() *BlacklistExemptionScheduler {
	interval := time.Minute
	if value := os.Getenv("BLACKLIST_EXEMPTION_INTERVAL"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			interval = parsed
		}
	}
	service := opsservice.NewBlacklistExemptionService()
	return NewBlacklistExemptionSchedulerWithDeps(BlacklistExemptionSchedulerDeps{
		Service:    service,
		Interval:   interval,
		Lifecycle:  schedulerx.NewLifecycle(),
		ExpireFunc: service.ExpireOverdue,
	})
}
