package auth

import (
	"sync"
	"time"
)

type attempt struct {
	count int
	start time.Time
}
type LoginLimiter struct {
	mu         sync.Mutex
	entries    map[string]attempt
	Clock      Clock
	Max        int
	Window     time.Duration
	MaxEntries int
	SweepEvery int
	mutations  int
}

func NewLoginLimiter(clock Clock) *LoginLimiter {
	return &LoginLimiter{entries: map[string]attempt{}, Clock: clock, Max: 5, Window: 15 * time.Minute, MaxEntries: 10_000, SweepEvery: 128}
}
func (l *LoginLimiter) Allow(key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.Clock.Now()
	a, ok := l.entries[key]
	if !ok {
		return true, 0
	}
	if now.Sub(a.start) >= l.Window {
		delete(l.entries, key)
		return true, 0
	}
	if a.count >= l.Max {
		retry := int(l.Window-now.Sub(a.start)+time.Second-1) / int(time.Second)
		if retry < 1 {
			retry = 1
		}
		return false, retry
	}
	return true, 0
}
func (l *LoginLimiter) Failure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.Clock.Now()
	l.mutations++
	if l.SweepEvery > 0 && l.mutations%l.SweepEvery == 0 {
		l.removeExpired(now)
	}
	a, ok := l.entries[key]
	if !ok || now.Sub(a.start) >= l.Window {
		a = attempt{start: now}
		ok = false
	}
	if !ok && l.MaxEntries > 0 && len(l.entries) >= l.MaxEntries {
		l.evictOldest()
	}
	a.count++
	l.entries[key] = a
}
func (l *LoginLimiter) Success(key string) { l.mu.Lock(); defer l.mu.Unlock(); delete(l.entries, key) }
func (l *LoginLimiter) removeExpired(now time.Time) {
	for key, value := range l.entries {
		if now.Sub(value.start) >= l.Window {
			delete(l.entries, key)
		}
	}
}
func (l *LoginLimiter) evictOldest() {
	var oldestKey string
	var oldest time.Time
	for key, value := range l.entries {
		if oldestKey == "" || value.start.Before(oldest) {
			oldestKey, oldest = key, value.start
		}
	}
	if oldestKey != "" {
		delete(l.entries, oldestKey)
	}
}
