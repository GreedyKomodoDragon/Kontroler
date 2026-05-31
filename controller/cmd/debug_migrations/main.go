package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"kontroler-controller/internal/db"
	"kontroler-controller/internal/db/migrations"

	_ "modernc.org/sqlite"
)

func main() {
	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer dbConn.Close()

	m := db.NewSQLiteMigrationManager(dbConn)
	// register migrations twice to simulate duplicate registration
	if err := migrations.RegisterMigrations(m, "sqlite"); err != nil {
		log.Fatalf("first register: %v", err)
	}
	if err := migrations.RegisterMigrations(m, "sqlite"); err != nil {
		log.Fatalf("second register: %v", err)
	}

	fmt.Println("Registered migrations. Calling MigrateUp")
	if err := m.MigrateUp(context.Background()); err != nil {
		log.Fatalf("MigrateUp: %v", err)
	}
	fmt.Println("MigrateUp OK")
}
