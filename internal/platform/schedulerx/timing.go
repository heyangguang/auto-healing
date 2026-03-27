package schedulerx

import "time"

func LastSyncStillCoolingDown(lastSyncAt *time.Time, interval time.Duration, now time.Time) bool {
	if lastSyncAt == nil || interval <= 0 {
		return false
	}
	return lastSyncAt.Add(interval).After(now)
}

func MaxDuration(first, second time.Duration) time.Duration {
	if first >= second {
		return first
	}
	return second
}
