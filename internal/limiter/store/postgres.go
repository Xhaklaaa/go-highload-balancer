package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/xhaklaaa/go-highload-balancer/internal/limiter"
)

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type PostgresStore struct {
	pool          *pgxpool.Pool
	defaultConfig limiter.RateConfig
}

func NewPostgresStore(cfg PostgresConfig, defaultConfig limiter.RateConfig) (*PostgresStore, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing postgres config: %w", err)
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	store := &PostgresStore{
		pool:          pool,
		defaultConfig: defaultConfig,
	}

	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("schema initialization failed: %w", err)
	}

	return store, nil
}

func (s *PostgresStore) initSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	createTableQuery := `
		CREATE TABLE IF NOT EXISTS rate_limits (
			client_id VARCHAR(255) PRIMARY KEY,
			capacity BIGINT NOT NULL,
			refill_rate DOUBLE PRECISION NOT NULL,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		);
		
		CREATE INDEX IF NOT EXISTS idx_client_id ON rate_limits (client_id);
	`

	_, err := s.pool.Exec(ctx, createTableQuery)
	return err
}

// GetConfig возвращает конфигурацию для клиента
func (s *PostgresStore) GetConfig(ctx context.Context, clientID string) (limiter.RateConfig, bool, error) {
	query := `
		SELECT capacity, refill_rate 
		FROM rate_limits 
		WHERE client_id = $1
	`

	var config limiter.RateConfig
	err := s.pool.QueryRow(ctx, query, clientID).Scan(
		&config.Capacity,
		&config.RefillRate,
	)

	if err != nil {
		return s.defaultConfig, false, nil
	}

	return config, true, nil
}

// UpdateConfig обновляет конфигурацию
func (s *PostgresStore) UpsertConfig(ctx context.Context, clientID string, config limiter.RateConfig) error {
	query := `
		INSERT INTO rate_limits (client_id, capacity, refill_rate)
		VALUES ($1, $2, $3)
		ON CONFLICT (client_id) 
		DO UPDATE SET 
			capacity = EXCLUDED.capacity,
			refill_rate = EXCLUDED.refill_rate,
			updated_at = NOW()
	`

	_, err := s.pool.Exec(ctx, query,
		clientID,
		config.Capacity,
		config.RefillRate,
	)

	return err
}

func (s *PostgresStore) DeleteConfig(ctx context.Context, clientID string) error {
	query := `DELETE FROM rate_limits WHERE client_id = $1`
	_, err := s.pool.Exec(ctx, query, clientID)
	return err
}

func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}
