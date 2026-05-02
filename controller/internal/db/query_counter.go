package db

import "sync/atomic"

var queryCounter int64

// ResetQueryCounter resets the internal query counter (for tests).
func ResetQueryCounter() {
	atomic.StoreInt64(&queryCounter, 0)
}

// GetQueryCount returns the current query count.
func GetQueryCount() int64 {
	return atomic.LoadInt64(&queryCounter)
}

func incrQueryCounter() {
	atomic.AddInt64(&queryCounter, 1)
}
