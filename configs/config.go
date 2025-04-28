package configs

import (
	"time"

	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/interfaces"
)

type AppConfig struct {
	Server struct {
		Port            int           `yaml:"port"`
		ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	} `yaml:"server"`

	Balancer struct {
		Algorithm      interfaces.AlgorithmType `yaml:"algorithm"`
		HealthCheckInt time.Duration            `yaml:"health_check_interval"`
		Backends       []string                 `yaml:"backends"`
	} `yaml:"balancer"`

	RateLimiter struct {
		DefaultCapacity int64   `yaml:"default_capacity"`
		DefaultRate     float64 `yaml:"default_rate"`
		StorageType     string  `yaml:"storage_type"`
	} `yaml:"rate_limiter"`
}
