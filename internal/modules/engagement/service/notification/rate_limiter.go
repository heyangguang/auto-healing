package notification

import (
	"sync"
	"time"
)

// RateLimiter 简单的滑动窗口限流器
type RateLimiter struct {
	mu       sync.RWMutex
	counters map[string]*windowCounter
}

// windowCounter 滑动窗口计数器
type windowCounter struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// NewRateLimiter 创建限流器
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		counters: make(map[string]*windowCounter),
	}
	// 启动清理协程
	go rl.cleanup()
	return rl
}

// Allow 检查是否允许请求，如果允许则记录并返回 true
func (r *RateLimiter) Allow(key string, limit int, window time.Duration) bool {
	if limit <= 0 {
		return true // 无限制
	}

	r.mu.Lock()
	counter, ok := r.counters[key]
	if !ok {
		counter = &windowCounter{
			timestamps: make([]time.Time, 0, limit),
		}
		r.counters[key] = counter
	}
	r.mu.Unlock()

	counter.mu.Lock()
	defer counter.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	// 清理过期的时间戳
	validTimestamps := make([]time.Time, 0, len(counter.timestamps))
	for _, ts := range counter.timestamps {
		if ts.After(cutoff) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	counter.timestamps = validTimestamps

	// 检查是否超限
	if len(counter.timestamps) >= limit {
		return false
	}

	// 记录当前请求
	counter.timestamps = append(counter.timestamps, now)
	return true
}

// GetCount 获取当前窗口内的请求数
func (r *RateLimiter) GetCount(key string, window time.Duration) int {
	r.mu.RLock()
	counter, ok := r.counters[key]
	r.mu.RUnlock()

	if !ok {
		return 0
	}

	counter.mu.Lock()
	defer counter.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	count := 0
	for _, ts := range counter.timestamps {
		if ts.After(cutoff) {
			count++
		}
	}
	return count
}

// cleanup 定期清理过期数据
func (r *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		cutoff := time.Now().Add(-5 * time.Minute)
		for key, counter := range r.counters {
			counter.mu.Lock()
			// 如果所有时间戳都过期了，删除这个 key
			allExpired := true
			for _, ts := range counter.timestamps {
				if ts.After(cutoff) {
					allExpired = false
					break
				}
			}
			if allExpired && len(counter.timestamps) > 0 {
				delete(r.counters, key)
			}
			counter.mu.Unlock()
		}
		r.mu.Unlock()
	}
}
