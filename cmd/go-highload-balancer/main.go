package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"

	"github.com/xhaklaaa/go-highload-balancer/internal/balancer"
	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/interfaces"
	"github.com/xhaklaaa/go-highload-balancer/internal/config"
	"github.com/xhaklaaa/go-highload-balancer/internal/limiter"
	"github.com/xhaklaaa/go-highload-balancer/internal/limiter/store"
	"github.com/xhaklaaa/go-highload-balancer/internal/logger"
	"github.com/xhaklaaa/go-highload-balancer/internal/migrations"
	"github.com/xhaklaaa/go-highload-balancer/internal/proxy"
	"github.com/xhaklaaa/go-highload-balancer/internal/server"
)

func main() {
	log := &logger.DefaultLogger{}

	cfg, err := config.Load("/configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := sql.Open("pgx", fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.RateLimiting.Postgres.User,
		cfg.RateLimiting.Postgres.Password,
		cfg.RateLimiting.Postgres.Host,
		cfg.RateLimiting.Postgres.Port,
		cfg.RateLimiting.Postgres.DBName,
	))
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := migrations.Run(ctx, db); err != nil {
		log.Fatalf("Migrations failed: %v", err)
	}

	// Инициализация балансировщика
	factory := balancer.NewStrategyFactory(log)
	lb, err := factory.New(
		interfaces.AlgorithmType(cfg.Balancing.Algorithm),
		cfg.Backends,
	)
	if err != nil {
		log.Fatalf("Failed to create balancer: %v", err)
	}

	if healthChecker, ok := lb.(interfaces.HealthChecker); ok {
		ctx := context.Background()
		go healthChecker.StartHealthChecks(ctx, 30*time.Second)
	} else {
		log.Warnf("Balancer does not support health checks")
	}

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
		cfg.RateLimiting.Enabled,
	)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
