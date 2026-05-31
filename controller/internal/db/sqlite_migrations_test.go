package db

import (
	"context"
	"database/sql"
	"testing"

	"kontroler-controller/internal/db/migrations"

	_ "modernc.org/sqlite"
)

func TestSQLiteMigrationsDoubleRegistration(t *testing.T) {
	// Use an in-memory SQLite DB to avoid filesystem side-effects
	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer dbConn.Close()

	m := NewSQLiteMigrationManager(dbConn)

	// Register migrations twice (simulate duplicate registration path)
	if err := migrations.RegisterMigrations(m, "sqlite"); err != nil {
		t.Fatalf("first RegisterMigrations failed: %v", err)
	}
	if err := migrations.RegisterMigrations(m, "sqlite"); err != nil {
		t.Fatalf("second RegisterMigrations failed: %v", err)
	}

	// MigrateUp should succeed despite the double registration
	if err := m.MigrateUp(context.Background()); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}
}
