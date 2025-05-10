package migrations

import (
	"context"
	"embed"
	"fmt"
)

//go:embed postgres/*.up.sql sqlite/*.up.sql
var migrationFiles embed.FS

type MigrationsManager interface {
	RegisterMigration(version int, description, up string)
	MigrateUp(ctx context.Context) error
}

// RegisterMigrations registers all migrations in order
func RegisterMigrations(manager MigrationsManager, dbType string) error {
	var migrationsPath string
	switch dbType {
	case "postgresql":
		migrationsPath = "postgres"
	case "sqlite":
		migrationsPath = "sqlite"
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	migrations := []struct {
		version     int
		description string
		filename    string
	}{
		{1, "Initial schema creation", migrationsPath + "/001_initial_schema.up.sql"},
		{2, "Add suspension capability to DAGs", migrationsPath + "/002_add_dag_suspension.up.sql"},
	}

	for _, m := range migrations {
		sql, err := migrationFiles.ReadFile(m.filename)
		if err != nil {
			return err
		}
		manager.RegisterMigration(m.version, m.description, string(sql))
	}

	return nil
}
