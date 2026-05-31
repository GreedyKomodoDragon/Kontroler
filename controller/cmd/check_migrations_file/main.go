package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"kontroler-controller/internal/db"
	"kontroler-controller/internal/db/migrations"
	_ "modernc.org/sqlite"
)

func main(){
	dbfile := "/tmp/kontroler_file_test.db"
	_ = os.Remove(dbfile)
	dbConn, err := sql.Open("sqlite", dbfile)
	if err!=nil{log.Fatal(err)}
	m := db.NewSQLiteMigrationManager(dbConn)
	if err := migrations.RegisterMigrations(m, "sqlite"); err != nil {
		log.Fatal(err)
	}
	if err := m.MigrateUp(context.Background()); err != nil {
		fmt.Printf("MigrateUp error: %v\n", err)
		return
	}
	fmt.Println("MigrateUp OK")
}
