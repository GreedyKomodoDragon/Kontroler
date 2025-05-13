package db

import (
	"context"
	"fmt"
	"kontroler-controller/internal/db/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresMigrationManager struct {
	pool       *pgxpool.Pool
	migrations []migration
}

func NewMigrationManager(pool *pgxpool.Pool) migrations.MigrationsManager {
	return &postgresMigrationManager{
		pool:       pool,
		migrations: []migration{},
	}
}

func (m *postgresMigrationManager) RegisterMigration(version int, description, up string) {
	m.migrations = append(m.migrations, migration{
		version:     version,
		description: description,
		up:          up,
	})
}

func (m *postgresMigrationManager) MigrateUp(ctx context.Context) error {
	// Create migrations table if it doesn't exist
	if _, err := m.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get applied migrations
	rows, err := tx.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC")
	if err != nil {
		return fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating migrations: %w", err)
	}

	// Apply pending migrations in order
	for _, m := range m.migrations {
		if !applied[m.version] {
			if _, err := tx.Exec(ctx, m.up); err != nil {
				if err == pgx.ErrNoRows {
					continue
				}
				return fmt.Errorf("failed to apply migration %d: %w", m.version, err)
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO schema_migrations (version, description)
				VALUES ($1, $2);
			`, m.version, m.description); err != nil {
				return fmt.Errorf("failed to record migration %d: %w", m.version, err)
			}
		}
	}

	return tx.Commit(ctx)
}
