package balancer

import (
	"net/url"
	"sync"

	"github.com/xhaklaaa/go-highload-balancer/internal/core"
)

type BackendPool struct {
	backends []*core.Backend
	mu       sync.RWMutex
}

func NewBackendPool(backendURLs []string) *BackendPool {
	pool := &BackendPool{
		backends: make([]*core.Backend, 0),
	}

	for _, rawURL := range backendURLs {
		u, _ := url.Parse(rawURL)
		pool.Add(&core.Backend{
			URL:     u,
			Healthy: true,
		})
	}

	return pool
}

func (p *BackendPool) Add(backend *core.Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backends = append(p.backends, backend)
}

func (p *BackendPool) Remove(url *url.URL) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, b := range p.backends {
		if b.URL.String() == url.String() {
			p.backends = append(p.backends[:i], p.backends[i+1:]...)
			return
		}
	}
}

func (p *BackendPool) GetAll() []*core.Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cpy := make([]*core.Backend, len(p.backends))
	copy(cpy, p.backends)
	return cpy
}

func (p *BackendPool) MarkHealthy(url *url.URL) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, b := range p.backends {
		if b.URL.String() == url.String() {
			b.Healthy = true
			return
		}
	}
}

func (p *BackendPool) MarkUnhealthy(url *url.URL) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, b := range p.backends {
		if b.URL.String() == url.String() {
			b.Healthy = false
			return
		}
	}
}

func (p *BackendPool) HealthyBackends() []*core.Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var healthy []*core.Backend
	for _, b := range p.backends {
		if b.Healthy {
			healthy = append(healthy, b)
		}
	}
	return healthy
}
