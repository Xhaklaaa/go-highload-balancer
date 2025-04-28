package balancer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"gopkg.in/go-playground/assert.v1"
)

type MockLogger struct {
	mu   sync.Mutex
	logs []string
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, "[INFO] "+fmt.Sprintf(format, args...))
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, "[WARN] "+fmt.Sprintf(format, args...))
}

func TestRoundRobinBalancer_NextBackend(t *testing.T) {
	tests := []struct {
		name         string
		backends     []string
		setup        func(*RoundRobinBalancer)
		expectedURLs []string
	}{
		{
			name:     "basic round robin cycle",
			backends: []string{"http://backend1", "http://backend2", "http://backend3"},
			setup: func(lb *RoundRobinBalancer) {
				atomic.StoreUint32(&lb.current, 2) // Начинаем с последнего
			},
			expectedURLs: []string{
				"http://backend1", // (2+1)%3=0
				"http://backend2", // (0+1)%3=1
				"http://backend3", // (1+1)%3=2
			},
		},
		{
			name:     "skip unavailable backend",
			backends: []string{"http://backend1", "http://backend2"},
			setup: func(lb *RoundRobinBalancer) {
				lb.MarkBackendStatus("http://backend2", false)
			},
			expectedURLs: []string{
				"http://backend1",
				"http://backend1", // Пропускаем backend2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &MockLogger{}
			lb := NewRoundRobinBalancer(tt.backends, logger)
			if tt.setup != nil {
				tt.setup(lb)
			}

			for _, expected := range tt.expectedURLs {
				selected := lb.NextBackend()
				if selected != expected {
					t.Errorf("Expected %s, got %s", expected, selected)
				}
			}
		})
	}
}

func TestRoundRobinBalancer_UnavailableBackends(t *testing.T) {
	logger := &MockLogger{}
	backends := []string{"http://backend1", "http://backend2"}
	lb := NewRoundRobinBalancer(backends, logger)

	// Помечаем все бэкенды как недоступные
	lb.MarkBackendStatus("http://backend1", false)
	lb.MarkBackendStatus("http://backend2", false)

	selected := lb.NextBackend()
	if selected != "" {
		t.Errorf("Expected empty string, got %s", selected)
	}

	// Проверяем логи:
	// 1. Изменение статуса backend1
	// 2. Изменение статуса backend2
	// 3. Предупреждение о недоступных бэкендах
	expectedLogs := 3
	if len(logger.logs) != expectedLogs {
		t.Errorf("Expected %d log entries, got %d. Logs: %v",
			expectedLogs,
			len(logger.logs),
			logger.logs)
	}
}

func TestRoundRobinConcurrent(t *testing.T) {
	backends := []string{"http://backend1", "http://backend2"}
	lb := NewRoundRobinBalancer(backends, &MockLogger{})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lb.NextBackend()
		}()
	}
	wg.Wait()

	// Проверяем отсутствие гонок данных
	final := lb.NextBackend()
	if final != "http://backend1" && final != "http://backend2" {
		t.Errorf("Unexpected final backend: %s", final)
	}
}

func TestBalancerWithUnhealthyBackend(t *testing.T) {
	pool := NewBackendPool()
	pool.AddBackend("http://invalid:8080")

	strategy := NewRoundRobin()
	proxy := NewReverseProxy(strategy, pool)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	proxy.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
