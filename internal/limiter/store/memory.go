package store

import (
	"context"
	"sync"

	"github.com/xhaklaaa/go-highload-balancer/internal/limiter"
)

type InMemoryStore struct {
	defaultConfig limiter.RateConfig
	clients       map[string]limiter.RateConfig
	mu            sync.RWMutex
}

func NewInMemoryStore(defaultConfig limiter.RateConfig) *InMemoryStore {
	return &InMemoryStore{
		defaultConfig: defaultConfig,
		clients:       make(map[string]limiter.RateConfig),
	}
}

func (s *InMemoryStore) GetConfig(ctx context.Context, clientID string) (limiter.RateConfig, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.clients[clientID]
	if !exists {
		return s.defaultConfig, false, nil
	}
	return config, true, nil
}

func (s *InMemoryStore) UpsertConfig(ctx context.Context, clientID string, config limiter.RateConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients[clientID] = config
	return nil
}

func (s *InMemoryStore) DeleteConfig(ctx context.Context, clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.clients, clientID)
	return nil
}

func (s *InMemoryStore) Close() error {
	return nil
}
