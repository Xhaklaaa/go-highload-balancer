package algorithms

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xhaklaaa/go-highload-balancer/internal/core"
	"github.com/xhaklaaa/go-highload-balancer/internal/logger"
)

// Реализует алгоритм балансировки с наименьшим количеством соединений
type LeastConnectionsBalancer struct {
	backends []*core.Backend
	indexMap map[string]int
	mu       sync.RWMutex
	logger   logger.Logger
	client   *http.Client
}

func NewLeastConnectionsBalancer(
	backendURLs []string,
	logger logger.Logger,
) *LeastConnectionsBalancer {
	lc := &LeastConnectionsBalancer{
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

		lc.backends = append(lc.backends, &core.Backend{
			URL:               u,
			Healthy:           true,
			ActiveConnections: 0,
		})
		lc.indexMap[u.String()] = i
	}

	return lc
}

func (lc *LeastConnectionsBalancer) Next(r *http.Request) (*url.URL, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	var (
		minConnections int64 = math.MaxInt64
		selected       *core.Backend
	)

	for _, backend := range lc.backends {
		connections := atomic.LoadInt64(&backend.ActiveConnections)
		if backend.IsHealthy() && connections < minConnections {
			minConnections = connections
			selected = backend
		}
	}

	if selected == nil {
		lc.logger.Warnf("All backends are unavailable")
		return nil, core.ErrNoAvailableBackend
	}

	atomic.AddInt64(&selected.ActiveConnections, 1)
	return selected.URL, nil
}

func (lc *LeastConnectionsBalancer) ReleaseConnection(urlStr string) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if idx, exists := lc.indexMap[urlStr]; exists {
		backend := lc.backends[idx]
		atomic.AddInt64(&backend.ActiveConnections, -1)
	}
}

func (lc *LeastConnectionsBalancer) MarkBackendStatus(urlStr string, healthy bool) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if idx, exists := lc.indexMap[urlStr]; exists {
		backend := lc.backends[idx]
		backend.SetHealthy(healthy)
		lc.logger.Infof("Backend status changed: %s -> %v", urlStr, healthy)
	}
}

func (lc *LeastConnectionsBalancer) StartHealthChecks(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lc.checkAllBackends()
		case <-ctx.Done():
			lc.logger.Infof("Health checks stopped")
			return
		}
	}
}

func (lc *LeastConnectionsBalancer) checkAllBackends() {
	var wg sync.WaitGroup

	for _, backend := range lc.backends {
		wg.Add(1)
		go func(be *core.Backend) {
			defer wg.Done()
			lc.checkBackendHealth(be)
		}(backend)
	}

	wg.Wait()
}

func (lc *LeastConnectionsBalancer) checkBackendHealth(backend *core.Backend) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", backend.URL.String()+"/health", nil)
	if err != nil {
		backend.SetHealthy(false)
		return
	}

	resp, err := lc.client.Do(req)
	if err != nil {
		backend.SetHealthy(false)
		return
	}
	defer resp.Body.Close()

	backend.SetHealthy(resp.StatusCode == http.StatusOK)
}

func (lc *LeastConnectionsBalancer) GetAll() []*core.Backend {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	backends := make([]*core.Backend, 0, len(lc.backends))
	backends = append(backends, lc.backends...)
	return backends
}
