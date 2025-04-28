package main

import (
	"context"
	"time"

	"github.com/xhaklaaa/go-highload-balancer/internal/balancer"
	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/interfaces"
	"github.com/xhaklaaa/go-highload-balancer/internal/config"
	"github.com/xhaklaaa/go-highload-balancer/internal/limiter"
	"github.com/xhaklaaa/go-highload-balancer/internal/limiter/store"
	"github.com/xhaklaaa/go-highload-balancer/internal/logger"
	"github.com/xhaklaaa/go-highload-balancer/internal/proxy"
	"github.com/xhaklaaa/go-highload-balancer/internal/server"
)

func main() {
	log := &logger.DefaultLogger{}

	cfg, err := config.Load(config.GetConfigPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация балансировщика
	factory := balancer.StrategyFactory(log)
	lb, err := factory.New(
		interfaces.AlgorithmType(cfg.Balancing.Algorithm),
		cfg.Backends,
		log,
	)
	if err != nil {
		log.Fatalf("Failed to create balancer: %v", err)
	}

	ctx := context.Background()
	go lb.StartHealthChecks(ctx, 30*time.Second)

	// Инициализация прокси
	proxyHandler := proxy.NewHandler(lb, log)

	// Инициализация rate limiter
	var rateStore limiter.ConfigStore
	defaultRateConfig := limiter.RateConfig{
		Capacity:   cfg.RateLimiting.Default.Capacity,
		RefillRate: cfg.RateLimiting.Default.Rate,
	}

	if cfg.RateLimiting.Enabled {
		if cfg.RateLimiting.Type == "postgres" {
			pgStore, err := store.NewPostgresStore(store.PostgresConfig{
				Host:     cfg.RateLimiting.Postgres.Host,
				Port:     cfg.RateLimiting.Postgres.Port,
				User:     cfg.RateLimiting.Postgres.User,
				Password: cfg.RateLimiting.Postgres.Password,
				DBName:   cfg.RateLimiting.Postgres.DBName,
				SSLMode:  "disable",
			}, defaultRateConfig)
			if err != nil {
				log.Fatalf("Failed to init PG store: %v", err)
			}
			rateStore = pgStore
		} else {
			rateStore = store.NewInMemoryStore(defaultRateConfig)
		}
	}

	rateLimiter := limiter.NewTokenBucket(rateStore, defaultRateConfig)

	srv := server.NewServer(
		lb,
		proxyHandler,
		cfg.Port,
		log,
		rateLimiter,
		rateStore,
	)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
