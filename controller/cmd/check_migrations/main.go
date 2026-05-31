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

func main(){
	dbfile := ":memory:"
	dbConn, err := sql.Open("sqlite", dbfile)
	if err!=nil{log.Fatal(err)}
	m := db.NewSQLiteMigrationManager(dbConn)
	if err := migrations.RegisterMigrations(m, "sqlite"); err != nil {
		log.Fatal(err)
	}
	// We can't access internal migration slice; try to run MigrateUp and report error
	if err := m.MigrateUp(context.Background()); err != nil {
		fmt.Printf("MigrateUp error: %v\n", err)
		return
	}
	fmt.Println("MigrateUp OK")
}
