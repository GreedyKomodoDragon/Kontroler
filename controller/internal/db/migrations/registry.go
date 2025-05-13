package migrations

import (
	"context"
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed postgresql/*.up.sql sqlite/*.up.sql
var migrationFiles embed.FS

type MigrationsManager interface {
	RegisterMigration(version int, description, up string)
	MigrateUp(ctx context.Context) error
}

type migrationInfo struct {
	version     int
	description string
	filename    string
}

// RegisterMigrations registers all migrations in order
func RegisterMigrations(manager MigrationsManager, dbType string) error {
	if dbType != "postgresql" && dbType != "sqlite" {
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	// Get all migration files for this database type
	entries, err := migrationFiles.ReadDir(dbType)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	re := regexp.MustCompile(`^(\d{3})_(.+)\.up\.sql$`)

	// Parse migration filenames to get version numbers
	migrations := make([]migrationInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := re.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		// Convert underscore description back to spaces
		desc := strings.ReplaceAll(matches[2], "-", " ")
		migrations = append(migrations, migrationInfo{
			version:     version,
			description: desc,
			filename:    fmt.Sprintf("%s/%s", dbType, entry.Name()),
		})
	}

	// Sort migrations by version number
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	// Register migrations in order
	for _, m := range migrations {
		sql, err := migrationFiles.ReadFile(m.filename)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", m.filename, err)
		}
		manager.RegisterMigration(m.version, m.description, string(sql))
	}

	return nil
}
