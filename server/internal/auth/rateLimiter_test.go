package auth

import (
	"testing"
	"time"
)

func TestRateLimiter_IsBlocked(t *testing.T) {
	rateLimiter := NewRateLimiter(3, 5*time.Minute)

	// Initially not blocked
	if rateLimiter.IsBlocked("testuser") {
		t.Error("Expected user to not be blocked initially")
	}

	// Record failures up to max attempts
	for i := 0; i < 2; i++ {
		blocked := rateLimiter.RecordFailure("testuser")
		if blocked {
			t.Errorf("Expected not to be blocked after %d failures", i+1)
		}
	}

	// Third failure should block
	blocked := rateLimiter.RecordFailure("testuser")
	if !blocked {
		t.Error("Expected to be blocked after 3 failures")
	}

	// Should now be blocked
	if !rateLimiter.IsBlocked("testuser") {
		t.Error("Expected user to be blocked")
	}
}

func TestRateLimiter_RecordSuccess(t *testing.T) {
	rateLimiter := NewRateLimiter(3, 5*time.Minute)

	// Record some failures
	rateLimiter.RecordFailure("testuser")
	rateLimiter.RecordFailure("testuser")

	// Record success should clear the attempts
	rateLimiter.RecordSuccess("testuser")

	// Should not be blocked anymore
	if rateLimiter.IsBlocked("testuser") {
		t.Error("Expected user to not be blocked after success")
	}

	// Should start fresh with count 1
	blocked := rateLimiter.RecordFailure("testuser")
	if blocked {
		t.Error("Expected not to be blocked after first failure following success")
	}
}

func TestRateLimiter_BlockDuration(t *testing.T) {
	rateLimiter := NewRateLimiter(2, 100*time.Millisecond)

	// Block the user
	rateLimiter.RecordFailure("testuser")
	rateLimiter.RecordFailure("testuser")

	// Should be blocked
	if !rateLimiter.IsBlocked("testuser") {
		t.Error("Expected user to be blocked")
	}

	// Wait for block to expire
	time.Sleep(150 * time.Millisecond)

	// Should no longer be blocked
	if rateLimiter.IsBlocked("testuser") {
		t.Error("Expected user to not be blocked after duration")
	}
}

func TestRateLimiter_DifferentUsers(t *testing.T) {
	rateLimiter := NewRateLimiter(2, 5*time.Minute)

	// Block one user
	rateLimiter.RecordFailure("user1")
	rateLimiter.RecordFailure("user1")

	// user1 should be blocked
	if !rateLimiter.IsBlocked("user1") {
		t.Error("Expected user1 to be blocked")
	}

	// user2 should not be blocked
	if rateLimiter.IsBlocked("user2") {
		t.Error("Expected user2 to not be blocked")
	}

	// user1 success should not affect user2
	rateLimiter.RecordSuccess("user1")

	if rateLimiter.IsBlocked("user1") {
		t.Error("Expected user1 to not be blocked after success")
	}

	if rateLimiter.IsBlocked("user2") {
		t.Error("Expected user2 to still not be blocked")
	}
}

func TestNewRateLimiter(t *testing.T) {
	maxAttempts := 5
	blockDuration := 10 * time.Minute

	rateLimiter := NewRateLimiter(maxAttempts, blockDuration)

	// Test that the rate limiter is properly initialized
	if rateLimiter.IsBlocked("testuser") {
		t.Error("Expected no users to be blocked initially")
	}

	// Test that it takes exactly maxAttempts to block
	for i := 0; i < maxAttempts-1; i++ {
		blocked := rateLimiter.RecordFailure("testuser")
		if blocked {
			t.Errorf("Expected not to be blocked after %d failures", i+1)
		}
	}

	// The maxAttempts-th failure should block
	blocked := rateLimiter.RecordFailure("testuser")
	if !blocked {
		t.Error("Expected to be blocked after maxAttempts failures")
	}
}
