package proxy

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/interfaces"
	"github.com/xhaklaaa/go-highload-balancer/internal/logger"
)

type ProxyHandler struct {
	balancer interfaces.Balancer
	client   *http.Client
	logger   logger.Logger
}

func NewProxyHandler(b interfaces.Balancer, logger logger.Logger) *ProxyHandler {
	return &ProxyHandler{
		balancer: b,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 100,
			},
		},
		logger: logger,
	}
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Infof("Received request", "method", r.Method, "path", r.URL.Path)

	maxRetries := len(h.balancer.GetAll())
	for i := 0; i < maxRetries; i++ {
		backendURL, err := h.balancer.Next(r)
		if err != nil {
			h.logger.Errorf("All backends unavailable")
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}

		var body io.Reader = r.Body
		if r.Body != nil {
			body = io.TeeReader(r.Body, &bytes.Buffer{})
		}

		req, err := http.NewRequest(r.Method, backendURL.String()+r.URL.Path, body)
		if err != nil {
			h.logger.Errorf("Error creating request", "backend", backendURL, "error", err)
			continue
		}

		copyHeaders(req.Header, r.Header)

		if lc, ok := h.balancer.(interface{ ReleaseConnection(url string) }); ok {
			defer lc.ReleaseConnection(backendURL.String())
		}

		resp, err := h.client.Do(req)
		if err != nil {
			h.logger.Errorf("Error reaching backend", "backend", backendURL, "error", err)
			h.balancer.MarkBackendStatus(backendURL.String(), false)
			continue
		}
		defer resp.Body.Close()

		copyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			h.logger.Errorf("Error copying response", "error", err)
		}
		return
	}

	http.Error(w, "Service unavailable after retries", http.StatusServiceUnavailable)
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
