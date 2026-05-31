package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 9100, "metrics port")
	flag.Parse()

	wp := os.Getenv("WORKERPOOL_NAME")
	if wp == "" {
		wp = "<unknown>"
	}

	log.Printf("starting dummy worker for workerpool=%s\n", wp)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "workerpool=%s\n", wp)
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%d", port)
	// run server in goroutine
	go func() {
		log.Printf("listening on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	// background work loop
	for {
		log.Printf("worker(%s): heartbeat %s", wp, time.Now().Format(time.RFC3339))
		time.Sleep(30 * time.Second)
	}
}
