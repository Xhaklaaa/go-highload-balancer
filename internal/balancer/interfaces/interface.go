package interfaces

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/xhaklaaa/go-highload-balancer/internal/core"
)

type AlgorithmType string

const (
	RoundRobin       AlgorithmType = "round_robin"
	LeastConnections AlgorithmType = "least_connections"
)

type Balancer interface {
	Next(*http.Request) (*url.URL, error)
	GetAll() []*core.Backend
	MarkBackendStatus(url string, alive bool)
}

type HealthChecker interface {
	StartHealthChecks(ctx context.Context, interval time.Duration)
}

type Backend struct {
	URL               *url.URL
	Healthy           bool
	ActiveConnections int64
}

type Logger interface {
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}
