package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"kontroler-controller/internal/kafka"
)

func main() {
	// Get Kubernetes configuration
	kubeConfig, err := ctrl.GetConfig()
	if err != nil {
		log.Fatalf("Failed to get Kubernetes config: %v", err)
	}

	// Create DagRun creator configuration
	config := &kafka.DagRunCreatorConfig{
		KafkaConfig: &kafka.KafkaConfig{
			Brokers:         []string{"localhost:9092"}, // Update with your Kafka brokers
			GroupID:         "kontroler-dagrun-creator",
			AutoOffsetReset: "latest", // or "earliest" to read from beginning
			CommitInterval:  time.Second * 5,
			MaxBytes:        10e6, // 10MB
			MinBytes:        10e3, // 10KB
			MaxWait:         time.Second * 1,
		},
		KubeConfig: kubeConfig,
		Namespace:  "default", // Default namespace for DagRuns
	}

	// Create the DagRun creator
	dagRunCreator, err := kafka.NewDagRunCreator(config)
	if err != nil {
		log.Fatalf("Failed to create DagRun creator: %v", err)
	}

	// Subscribe to DagRun trigger topics
	topics := []string{
		"dagrun-triggers",
	}

	if err := dagRunCreator.Subscribe(topics...); err != nil {
		log.Fatalf("Failed to subscribe to topics: %v", err)
	} // Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming in a goroutine
	go func() {
		if err := dagRunCreator.Start(ctx); err != nil {
			log.Printf("DagRun creator stopped with error: %v", err)
		}
	}()

	log.Println("DagRun creator started. Listening for events to create DagRuns...")
	log.Println("Expected event format:")
	log.Println(`{
	"id": "unique-event-id",
	"type": "dagrun.trigger",
	"source": "scheduler|webhook|manual",
	"timestamp": 1640995200,
	"dagName": "my-dag",
	"runName": "my-dag-run-1", 
	"namespace": "default",
	"parameters": {
		"param1": "value1",
		"param2": "value2"
	}
}`)

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received")

	// Cancel context to stop consumer
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)

	// Stop the consumer
	if err := dagRunCreator.Stop(); err != nil {
		log.Printf("Error stopping DagRun creator: %v", err)
	}

	log.Println("DagRun creator stopped successfully")
}
