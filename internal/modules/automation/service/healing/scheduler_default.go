package healing

import "sync"

var (
	defaultSchedulerOnce sync.Once
	defaultScheduler     *Scheduler
)

func DefaultScheduler() *Scheduler {
	defaultSchedulerOnce.Do(func() {
		defaultScheduler = NewScheduler()
	})
	return defaultScheduler
}
