package provider

import "time"

func lastSyncStillCoolingDown(lastSyncAt *time.Time, interval time.Duration, now time.Time) bool {
	if lastSyncAt == nil || interval <= 0 {
		return false
	}
	return lastSyncAt.Add(interval).After(now)
}
