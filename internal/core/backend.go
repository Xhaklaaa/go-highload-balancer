package core

import (
	"errors"
	"net/url"
	"sync"
)

var (
	ErrNoAvailableBackend = errors.New("no available backends")
	ErrInvalidAlgorithm   = errors.New("invalid load balancing algorithm")
)

type Backend struct {
	URL               *url.URL
	IsAlive           bool
	Healthy           bool
	ActiveConnections int64
	mu                sync.RWMutex
}

func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.IsAlive = alive
}

func (b *Backend) IsAvailable() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.IsAlive
}

func (b *Backend) SetHealthy(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Healthy = healthy
}

func (b *Backend) IsHealthy() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Healthy
}
