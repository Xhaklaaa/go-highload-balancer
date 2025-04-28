package limiter

import (
	"sync"
	"time"
)

type bucket struct {
	capacity   int64
	rate       float64
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

func newBucket(capacity int64, rate float64) *bucket {
	return &bucket{
		capacity:   capacity,
		rate:       rate,
		tokens:     float64(capacity),
		lastRefill: time.Now(),
	}
}

func (b *bucket) allow(tokens int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > float64(b.capacity) {
		b.tokens = float64(b.capacity)
	}
	b.lastRefill = now

	if b.tokens >= float64(tokens) {
		b.tokens -= float64(tokens)
		return true
	}
	return false
}

func (b *bucket) refill() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > float64(b.capacity) {
		b.tokens = float64(b.capacity)
	}
	b.lastRefill = now
}
