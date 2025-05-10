package db

import (
	"context"
	"database/sql"
	"fmt"
	"kontroler-controller/internal/db/migrations"
)

type sqliteMigrationManager struct {
	db         *sql.DB
	migrations []migration
	initScript string
}

type migration struct {
	version     int
	description string
	up          string
}

func NewSQLiteMigrationManager(db *sql.DB) migrations.MigrationsManager {
	return &sqliteMigrationManager{
		db:         db,
		migrations: []migration{},
	}
}

func (m *sqliteMigrationManager) RegisterMigration(version int, description, up string) {
	m.migrations = append(m.migrations, migration{
		version:     version,
		description: description,
		up:          up,
	})
}

func (m *sqliteMigrationManager) MigrateUp(ctx context.Context) error {
	// Create migrations table if it doesn't exist
	if _, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get applied migrations
	rows, err := tx.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC")
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
		fmt.Println("Applying migration:", m.version, m.description)
		if !applied[m.version] {
			if _, err := tx.ExecContext(ctx, m.up); err != nil {
				return fmt.Errorf("failed to apply migration %d: %w", m.version, err)
			}

			if _, err := tx.ExecContext(ctx, `
				INSERT INTO schema_migrations (version, description)
				VALUES (?, ?);
			`, m.version, m.description); err != nil {
				return fmt.Errorf("failed to record migration %d: %w", m.version, err)
			}
		}
	}

	return tx.Commit()
}
