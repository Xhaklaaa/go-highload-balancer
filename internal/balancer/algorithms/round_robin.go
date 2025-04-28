package algorithms

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xhaklaaa/go-highload-balancer/internal/core"
	"github.com/xhaklaaa/go-highload-balancer/internal/logger"
)

type RoundRobin struct {
	backends []*core.Backend
}

func NewRoundRobin(backends []*core.Backend) *RoundRobin {
	return &RoundRobin{
		backends: backends,
	}
}

// Backend представляет бэкенд сервер с состоянием здоровья

type RoundRobinBalancer struct {
	mu       sync.RWMutex
	backends []*core.Backend
	current  uint32
	indexMap map[string]int
	logger   logger.Logger
	client   *http.Client
}

func NewRoundRobinBalancer(
	backendURLs []string,
	logger logger.Logger,
) *RoundRobinBalancer {
	rrb := &RoundRobinBalancer{
		indexMap: make(map[string]int),
		logger:   logger,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	for i, rawURL := range backendURLs {
		u, err := url.Parse(rawURL)
		if err != nil {
			logger.Warnf("Invalid backend URL: %s, error: %v", rawURL, err)
			continue
		}

		backend := &core.Backend{
			URL:     u,
			Healthy: true,
		}

		rrb.backends = append(rrb.backends, backend)
		rrb.indexMap[u.String()] = i
	}

	return rrb
}

func (b *RoundRobinBalancer) Next(r *http.Request) (*url.URL, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.backends) == 0 {
		b.logger.Warnf("No available backends")
		return nil, core.ErrNoAvailableBackend
	}

	start := atomic.LoadUint32(&b.current)
	next := start

	for i := 0; i < len(b.backends); i++ {
		next = (next + 1) % uint32(len(b.backends))
		backend := b.backends[next]

		if backend.IsHealthy() {
			atomic.StoreUint32(&b.current, next)
			return backend.URL, nil
		}
	}

	b.logger.Warnf("All backends are unavailable")
	return nil, core.ErrNoAvailableBackend
}

func (b *RoundRobinBalancer) MarkBackendStatus(url string, alive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if idx, exists := b.indexMap[url]; exists {
		b.backends[idx].SetAlive(alive)
		b.logger.Infof("Backend status changed: %s -> %v", url, alive)
	}
}

func (b *RoundRobinBalancer) StartHealthChecks(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.checkAllBackends()
		case <-ctx.Done():
			b.logger.Infof("Health checks stopped")
			return
		}
	}
}

func (b *RoundRobinBalancer) checkAllBackends() {
	var wg sync.WaitGroup

	for _, backend := range b.backends {
		wg.Add(1)
		go func(be *core.Backend) {
			defer wg.Done()
			b.checkBackendHealth(be)
		}(backend)
	}

	wg.Wait()
}

func (b *RoundRobinBalancer) checkBackendHealth(backend *core.Backend) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", backend.URL.String()+"/health", nil)
	if err != nil {
		backend.SetHealthy(false)
		return
	}

	resp, err := b.client.Do(req)
	if err != nil {
		backend.SetHealthy(false)
		return
	}
	defer resp.Body.Close()

	backend.SetHealthy(resp.StatusCode == http.StatusOK)
}
