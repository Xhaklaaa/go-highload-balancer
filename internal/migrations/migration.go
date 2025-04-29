package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

type Migration struct {
	Version string
	Up      func(*sql.Tx) error
}

var migrations = []Migration{
	{
		Version: "2024053001",
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS clients (
					client_id VARCHAR(255) PRIMARY KEY,
					capacity BIGINT NOT NULL,
					rate_per_sec BIGINT NOT NULL,
					created_at TIMESTAMP DEFAULT NOW(),
					updated_at TIMESTAMP DEFAULT NOW()
				)
			`)
			return err
		},
	},
	{
		Version: "2024053002",
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_client_id 
				ON clients (client_id)
			`)
			return err
		},
	},
}

func Run(ctx context.Context, db *sql.DB) error {
	// Создаем таблицу для отслеживания миграций
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(14) PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Получаем список выполненных миграций
	rows, err := db.QueryContext(ctx,
		"SELECT version FROM schema_migrations ORDER BY version DESC")
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return err
		}
		applied[version] = true
	}

	// Применяем новые миграции
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		log.Printf("Applying migration: %s", m.Version)
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if err := m.Up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", m.Version, err)
		}

		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version) VALUES ($1)",
			m.Version,
		); err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}
