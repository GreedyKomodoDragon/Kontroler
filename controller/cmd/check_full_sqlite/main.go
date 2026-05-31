package main

import (
	"context"
	"fmt"
	"log"

	"kontroler-controller/internal/db"
	cron "github.com/robfig/cron/v3"
)

func main(){
	parser := cron.NewParser(cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow)
	cfg := &db.SQLiteConfig{DBPath: ":memory:"}
	dm, dbConn, err := db.NewSqliteManagerWithMetrics(context.Background(), &parser, cfg)
	if err != nil { log.Fatalf("NewSqliteManagerWithMetrics error: %v", err) }
	defer dbConn.Close()
	fmt.Println("Created manager; calling InitaliseDatabase")
	if err := dm.InitaliseDatabase(context.Background()); err != nil {
		log.Fatalf("InitaliseDatabase error: %v", err)
	}
	fmt.Println("InitaliseDatabase OK")
}
