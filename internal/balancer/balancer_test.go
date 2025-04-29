package balancer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/algorithms"
	"github.com/xhaklaaa/go-highload-balancer/internal/core"
	"gopkg.in/go-playground/assert.v1"
)

// MockLogger реализует интерфейс Logger для тестов
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

// Тест RoundRobin алгоритма
func TestRoundRobinBalancer_NextBackend(t *testing.T) {
	tests := []struct {
		name         string
		backends     []string
		setup        func(*algorithms.RoundRobinBalancer)
		expectedURLs []string
	}{
		{
			name:     "basic round robin cycle",
			backends: []string{"http://backend1", "http://backend2", "http://backend3"},
			setup: func(lb *algorithms.RoundRobinBalancer) {
				atomic.StoreUint32(&lb.Current, 2) // Начинаем с последнего
			},
			expectedURLs: []string{
				"http://backend1", // (2+1)%3=0
				"http://backend2", // (0+1)%3=1
				"http://backend3", // (1+1)%3=2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &MockLogger{}
			lb := algorithms.NewRoundRobinBalancer(tt.backends, logger)
			if tt.setup != nil {
				tt.setup(lb)
			}

			for _, expected := range tt.expectedURLs {
				selected, _ := lb.Next(nil)
				if selected.String() != expected {
					t.Errorf("Expected %s, got %s", expected, selected)
				}
			}
		})
	}
}

// Тест недоступных бэкендов
func TestRoundRobinBalancer_UnavailableBackends(t *testing.T) {
	logger := &MockLogger{}
	backends := []string{"http://backend1", "http://backend2"}
	lb := algorithms.NewRoundRobinBalancer(backends, logger)

	// Помечаем все бэкенды как недоступные
	lb.MarkBackendStatus("http://backend1", false)
	lb.MarkBackendStatus("http://backend2", false)

	_, err := lb.Next(nil)
	if err == nil {
		t.Error("Expected error for no available backends")
	}
}

// Тест на конкурентный доступ
func TestRoundRobinConcurrent(t *testing.T) {
	backends := []string{"http://backend1", "http://backend2"}
	lb := algorithms.NewRoundRobinBalancer(backends, &MockLogger{})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lb.Next(nil)
		}()
	}
	wg.Wait()
}

// Тест ReverseProxy с недоступными бэкендами
func TestBalancerWithUnhealthyBackend(t *testing.T) {
	// Создаем тестовый бэкенд
	backendURL, _ := url.Parse("http://invalid:8080")
	pool := core.NewBackendPool([]*url.URL{backendURL})

	// Инициализация балансировщика
	strategy := algorithms.NewRoundRobinBalancer([]string{"http://invalid:8080"}, &MockLogger{})

	// Создаем прокси
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	handler := NewProxyHandler(strategy, &MockLogger{})

	// Тестовый запрос
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
