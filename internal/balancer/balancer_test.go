package balancer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/algorithms"
	"github.com/xhaklaaa/go-highload-balancer/internal/proxy"
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

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, "[ERROR] "+fmt.Sprintf(format, args...))
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, "[FATAL] "+fmt.Sprintf(format, args...))
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
				atomic.StoreUint32(&lb.Current, 2)
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

func TestRoundRobinBalancer_UnavailableBackends(t *testing.T) {
	logger := &MockLogger{}
	backends := []string{"http://backend1", "http://backend2"}
	lb := algorithms.NewRoundRobinBalancer(backends, logger)

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

	strategy := algorithms.NewRoundRobinBalancer([]string{"http://invalid:8080"}, &MockLogger{})

	handler := proxy.NewProxyHandler(strategy, &MockLogger{})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
