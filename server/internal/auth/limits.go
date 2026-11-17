package auth

import (
	"sync"
	"time"
)

type limitEntry struct {
	count, failures      int
	start, next, expires time.Time
}
type limiter struct {
	mu      sync.Mutex
	entries map[string]limitEntry
}

func newLimiter() *limiter { return &limiter{entries: map[string]limitEntry{}} }
func (l *limiter) clean(t time.Time) {
	for k, v := range l.entries {
		if t.After(v.expires) {
			delete(l.entries, k)
		}
	}
}
func (l *limiter) allow(key string, n int, window, cooldown time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := time.Now()
	l.clean(t)
	v := l.entries[key]
	if t.Before(v.next) {
		return false
	}
	if t.Sub(v.start) >= window {
		v = limitEntry{start: t}
	}
	if v.count >= n || len(l.entries) >= 10000 && v.count == 0 {
		return false
	}
	v.count++
	v.next = t.Add(cooldown)
	v.expires = t.Add(window)
	l.entries[key] = v
	return true
}
func (l *limiter) loginAllowed(email string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := time.Now()
	l.clean(t)
	return !t.Before(l.entries["login:"+email].next)
}
func (l *limiter) loginResult(email string, success bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := "login:" + email
	t := time.Now()
	v := l.entries[key]
	if success {
		delete(l.entries, key)
		return
	}
	if t.Before(v.next) {
		return
	}
	if len(l.entries) >= 10000 && v.failures == 0 {
		return
	}
	v.failures++
	v.expires = t.Add(time.Hour)
	if v.failures >= 5 {
		delay := 30 * time.Second
		for i := 5; i < v.failures && delay < 15*time.Minute; i++ {
			delay *= 2
		}
		if delay > 15*time.Minute {
			delay = 15 * time.Minute
		}
		v.next = t.Add(delay)
	}
	l.entries[key] = v
}
