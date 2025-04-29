package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Checker struct {
	client   *http.Client
	interval time.Duration
	stopChan chan struct{}
}

// Balancer интерфейс для взаимодействия с балансировщиком
type Balancer interface {
	GetBackends() []*Backend
	MarkBackendStatus(url string, alive bool)
}

type Backend struct {
	URL     string
	IsAlive bool
}

func NewChecker(interval time.Duration) *Checker {
	return &Checker{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

func (c *Checker) Start(b Balancer) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkBackends(b)
		case <-c.stopChan:
			return
		}
	}
}

func (c *Checker) Stop() {
	close(c.stopChan)
}

func (c *Checker) checkBackends(b Balancer) {
	backends := b.GetBackends()

	var wg sync.WaitGroup
	for _, backend := range backends {
		wg.Add(1)
		go func(be *Backend) {
			defer wg.Done()
			c.checkSingleBackend(b, be)
		}(backend)
	}
	wg.Wait()
}

func (c *Checker) checkSingleBackend(b Balancer, be *Backend) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/health", be.URL),
		nil,
	)
	if err != nil {
		b.MarkBackendStatus(be.URL, false)
		return
	}

	resp, err := c.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		b.MarkBackendStatus(be.URL, false)
		return
	}
	defer resp.Body.Close()

	b.MarkBackendStatus(be.URL, true)
}
