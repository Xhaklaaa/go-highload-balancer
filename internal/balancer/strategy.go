package balancer

import (
	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/algorithms"
	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/interfaces"
	"github.com/xhaklaaa/go-highload-balancer/internal/core"
)

type StrategyFactory struct {
	Logger interfaces.Logger
}

type Strategy interface {
	NextBackend() (*core.Backend, error)
	SetBackends(backends []*core.Backend)
}

func (f *StrategyFactory) New(
	algorithm interfaces.AlgorithmType,
	backendURLs []string,
) (interfaces.Balancer, error) {

	switch algorithm {
	case interfaces.RoundRobin:
		return algorithms.NewRoundRobinBalancer(backendURLs, f.Logger), nil
	case interfaces.LeastConnections:
		return algorithms.NewLeastConnectionsBalancer(backendURLs, f.Logger), nil
	default:
		return nil, core.ErrInvalidAlgorithm
	}
}
