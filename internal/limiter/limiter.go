package limiter

import (
	"context"
	"sync"
	"time"
)

type RateLimiter interface {
	Allow(ctx context.Context, clientID string) bool
	Stop() error
}

type TokenBucket struct {
	buckets     *sync.Map
	store       ConfigStore
	stopChan    chan struct{}
	mu          sync.RWMutex
	defaultRate RateConfig
}

func NewTokenBucket(store ConfigStore, defaultRate RateConfig) *TokenBucket {
	tb := &TokenBucket{
		buckets:     &sync.Map{},
		store:       store,
		stopChan:    make(chan struct{}),
		defaultRate: defaultRate,
	}
	go tb.backgroundRefill()
	return tb
}

func (tb *TokenBucket) Allow(ctx context.Context, clientID string) bool {
	config, exists, err := tb.store.GetConfig(ctx, clientID)
	if err != nil {
		tb.mu.RLock()
		config = tb.defaultRate
		tb.mu.RUnlock()
	} else if !exists {
		config = tb.defaultRate
	}

	val, _ := tb.buckets.LoadOrStore(clientID, newBucket(
		config.Capacity,
		config.RefillRate,
	))

	return val.(*bucket).allow(1)
}

func (tb *TokenBucket) Stop() error {
	close(tb.stopChan)
	if storeWithCloser, ok := tb.store.(interface{ Close() error }); ok {
		return storeWithCloser.Close()
	}
	return nil
}

func (tb *TokenBucket) backgroundRefill() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tb.refillBuckets()
		case <-tb.stopChan:
			return
		}
	}
}

func (tb *TokenBucket) refillBuckets() {
	tb.buckets.Range(func(key, value interface{}) bool {
		value.(*bucket).refill()
		return true
	})
}
