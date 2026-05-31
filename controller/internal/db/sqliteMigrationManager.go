package db

import (
	"context"
	"database/sql"
	"fmt"
	"kontroler-controller/internal/db/migrations"

	log "sigs.k8s.io/controller-runtime/pkg/log"
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
	// Avoid registering the same version multiple times
	for _, existing := range m.migrations {
		if existing.version == version {
			log.Log.Info("skipping duplicate migration registration", "version", version, "description", description)
			return
		}
	}

	// Debug: print registration to help diagnose duplicate migrations
	log.Log.Info("registering migration", "version", version, "description", description)
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

	// Debug output of registered and applied migrations
	log.Log.Info("applied migrations map", "applied", applied)
	log.Log.Info("registered migrations list")
	for _, mm := range m.migrations {
		log.Log.Info("registered migration", "version", mm.version, "description", mm.description)
	}

	// Apply pending migrations in order
	for _, mm := range m.migrations {
		log.Log.Info("checking migration", "version", mm.version)
		if !applied[mm.version] {
			log.Log.Info("applying migration", "version", mm.version)
			// Execute migration SQL
			if _, err := tx.ExecContext(ctx, mm.up); err != nil {
				fmt.Printf("MIGRATION-ERROR: failed to exec migration %d: %v\n", mm.version, err)
				return fmt.Errorf("failed to apply migration %d: %w", mm.version, err)
			}

			// Before recording, query to ensure not present (defensive)
			var cnt int
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(1) FROM schema_migrations WHERE version = ?", mm.version).Scan(&cnt); err != nil {
				fmt.Printf("MIGRATION-ERROR: failed to query existing migration %d: %v\n", mm.version, err)
				return fmt.Errorf("failed to check existing migration %d: %w", mm.version, err)
			}
			fmt.Printf("MIGRATION-DBG: existing count for migration %d = %d\n", mm.version, cnt)
			if cnt > 0 {
				return fmt.Errorf("migration %d already recorded (cnt=%d)", mm.version, cnt)
			}

			// Insert record
			fmt.Printf("MIGRATION-DBG: inserting record for migration %d\n", mm.version)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO schema_migrations (version, description)
				VALUES (?, ?);
			`, mm.version, mm.description); err != nil {
				fmt.Printf("MIGRATION-ERROR: failed to insert migration record %d: %v\n", mm.version, err)
				return fmt.Errorf("failed to record migration %d: %w", mm.version, err)
			}
		}
	}

	return tx.Commit()
}
