package proxy

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/interfaces"
	"github.com/xhaklaaa/go-highload-balancer/internal/logger"
)

type Handler struct {
	balancer interfaces.Balancer
	client   *http.Client
	logger   logger.Logger
}

func NewHandler(b interfaces.Balancer, logger *log.Logger) *Handler {
	return &Handler{
		balancer: b,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Сохраняем тело запроса для повторного использования
	var bodyBytes []byte
	if r.Body != nil {
		var buf bytes.Buffer
		tee := io.TeeReader(r.Body, &buf)
		bodyBytes, _ = io.ReadAll(tee)
		r.Body.Close()
		r.Body = io.NopCloser(&buf)
	}

	// Пытаемся найти рабочий бэкенд за N попыток (N = количество бэкендов)
	backends := h.balancer.GetAll()
	maxRetries := len(backends)
	for i := 0; i < maxRetries; i++ {
		backendURL, err := h.balancer.Next(r)
		if err != nil {
			h.logger.Warnf("No available backend")
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}

		// Собираем URL для бэкенда
		targetURL := backendURL.ResolveReference(&url.URL{
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		})

		// Создаем новый запрос
		req, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(bodyBytes))
		if err != nil {
			h.logger.Errorf("Error creating request to backend %s: %v", backendURL, err)
			continue
		}

		// Копируем заголовки
		req.Header = r.Header.Clone()

		// Выполняем запрос
		resp, err := h.client.Do(req)
		if err != nil {
			h.logger.Errorf("Error reaching backend %s: %v", backendURL, err)
			h.balancer.MarkBackendStatus(backendURL.String(), false)
			continue
		}
		defer resp.Body.Close()

		// Копируем заголовки ответа
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}

		// Отправляем статус и тело ответа
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			h.logger.Errorf("Error copying response body: %v", err)
		}
		return
	}

	http.Error(w, "All backends unavailable after retries", http.StatusServiceUnavailable)
}
