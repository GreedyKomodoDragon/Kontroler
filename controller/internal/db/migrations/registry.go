package migrations

import (
	"context"
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	log "sigs.k8s.io/controller-runtime/pkg/log"
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
		filename := fmt.Sprintf("%s/%s", dbType, entry.Name())
		log.Log.Info("found migration file", "filename", filename, "version", version, "desc", desc)
		migrations = append(migrations, migrationInfo{
			version:     version,
			description: desc,
			filename:    filename,
		})
	}

	// Sort migrations by version number
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	// Register migrations in order, but avoid duplicate versions being registered twice
	seen := make(map[int]bool)
	for _, m := range migrations {
		if seen[m.version] {
			// Log and skip duplicate migration files with the same version
			fmt.Printf("debug: skipping duplicate migration version=%d filename=%s\n", m.version, m.filename)
			continue
		}

		sql, err := migrationFiles.ReadFile(m.filename)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", m.filename, err)
		}
		fmt.Printf("debug: registering migration version=%d filename=%s\n", m.version, m.filename)
		manager.RegisterMigration(m.version, m.description, string(sql))
		seen[m.version] = true
	}

	return nil
}
