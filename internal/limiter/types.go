package limiter

import "context"

type RateConfig struct {
	Capacity   int64   `json:"capacity"`
	RefillRate float64 `json:"refill_rate"`
}

// ConfigStore интерфейс хранилища конфигов
type ConfigStore interface {
	GetConfig(ctx context.Context, clientID string) (RateConfig, bool, error)
	UpsertConfig(ctx context.Context, clientID string, config RateConfig) error
	DeleteConfig(ctx context.Context, clientID string) error
	Close() error
}
