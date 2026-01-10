package auth

import (
	"sync"
	"time"
)

type LoginAttempt struct {
	Count    int
	LastTime time.Time
	Blocked  bool
}

type RateLimiter struct {
	attempts      map[string]*LoginAttempt
	mutex         sync.RWMutex
	maxAttempts   int
	blockDuration time.Duration
}

func NewRateLimiter(maxAttempts int, blockDuration time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts:      make(map[string]*LoginAttempt),
		maxAttempts:   maxAttempts,
		blockDuration: blockDuration,
	}
}

func (r *RateLimiter) IsBlocked(identifier string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	attempt, exists := r.attempts[identifier]
	if !exists {
		return false
	}

	if attempt.Blocked {
		if time.Since(attempt.LastTime) > r.blockDuration {
			delete(r.attempts, identifier)
			return false
		}
		return true
	}

	return false
}

func (r *RateLimiter) RecordFailure(identifier string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	attempt, exists := r.attempts[identifier]

	if !exists {
		r.attempts[identifier] = &LoginAttempt{
			Count:    1,
			LastTime: now,
			Blocked:  false,
		}
		return false
	}

	// Reset count if last attempt was more than 15 minutes ago
	if now.Sub(attempt.LastTime) > 15*time.Minute {
		attempt.Count = 1
		attempt.LastTime = now
		attempt.Blocked = false
		return false
	}

	attempt.Count++
	attempt.LastTime = now

	if attempt.Count >= r.maxAttempts {
		attempt.Blocked = true
		return true
	}

	return false
}

func (r *RateLimiter) RecordSuccess(identifier string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.attempts, identifier)
}
