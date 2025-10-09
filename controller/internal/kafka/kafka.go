package kafka

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// EventConsumer defines the interface for consuming events from Kafka topics
type EventConsumer interface {
	// Start begins consuming messages from the configured topics
	Start(ctx context.Context) error

	// Stop gracefully stops the consumer
	Stop() error

	// Subscribe adds topics to consume from
	Subscribe(topics ...string) error

	// IsRunning returns true if the consumer is currently running
	IsRunning() bool
}

// KafkaConfig holds the configuration for Kafka consumer
type KafkaConfig struct {
	// Brokers is a list of Kafka broker addresses
	Brokers []string

	// GroupID is the consumer group ID
	GroupID string

	// Topics is a list of topics to consume from
	Topics []string

	// AutoOffsetReset determines where to start reading when there's no initial offset
	// Options: "earliest", "latest"
	AutoOffsetReset string

	// CommitInterval is how often to commit offsets
	CommitInterval time.Duration

	// MaxBytes is the maximum number of bytes to fetch per request
	MaxBytes int

	// MinBytes is the minimum number of bytes to fetch per request
	MinBytes int

	// MaxWait is the maximum amount of time to wait for new data
	MaxWait time.Duration
}

// DefaultKafkaConfig returns a default configuration
func DefaultKafkaConfig() *KafkaConfig {
	return &KafkaConfig{
		Brokers:         []string{"localhost:9092"},
		GroupID:         "kontroler-consumer-group",
		AutoOffsetReset: "latest",
		CommitInterval:  time.Second * 10,
		MaxBytes:        10e6, // 10MB
		MinBytes:        10e3, // 10KB
		MaxWait:         time.Second * 1,
	}
}

// Consumer represents a Kafka consumer instance
type Consumer struct {
	config   *KafkaConfig
	readers  map[string]*kafka.Reader
	running  bool
	stopChan chan struct{}
	doneChan chan struct{}
}

// NewConsumer creates a new Kafka consumer with the given configuration
func NewConsumer(config *KafkaConfig) *Consumer {
	if config == nil {
		config = DefaultKafkaConfig()
	}

	return &Consumer{
		config:   config,
		readers:  make(map[string]*kafka.Reader),
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

// Subscribe adds topics to consume from
func (c *Consumer) Subscribe(topics ...string) error {
	if c.running {
		return fmt.Errorf("cannot subscribe to topics while consumer is running")
	}

	for _, topic := range topics {
		if _, exists := c.readers[topic]; exists {
			log.Printf("Topic %s is already subscribed", topic)
			continue
		}

		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:        c.config.Brokers,
			GroupID:        c.config.GroupID,
			Topic:          topic,
			CommitInterval: c.config.CommitInterval,
			StartOffset:    c.getStartOffset(),
			MaxBytes:       c.config.MaxBytes,
			MinBytes:       c.config.MinBytes,
			MaxWait:        c.config.MaxWait,
		})

		c.readers[topic] = reader
		log.Printf("Subscribed to topic: %s", topic)
	}

	return nil
}

// Start begins consuming messages from all subscribed topics
func (c *Consumer) Start(ctx context.Context) error {
	if c.running {
		return fmt.Errorf("consumer is already running")
	}

	if len(c.readers) == 0 {
		return fmt.Errorf("no topics subscribed")
	}

	c.running = true

	// Start a goroutine for each topic reader
	for topic, reader := range c.readers {
		go c.consumeFromTopic(ctx, topic, reader)
	}

	log.Printf("Kafka consumer started, consuming from %d topics", len(c.readers))

	// Wait for stop signal or context cancellation
	select {
	case <-ctx.Done():
		log.Println("Context cancelled, stopping consumer")
		return c.Stop()
	case <-c.stopChan:
		log.Println("Stop signal received")
	}

	close(c.doneChan)
	return nil
}

// Stop gracefully stops the consumer
func (c *Consumer) Stop() error {
	if !c.running {
		return nil
	}

	log.Println("Stopping Kafka consumer...")

	c.running = false
	close(c.stopChan)

	// Close all readers
	for topic, reader := range c.readers {
		if err := reader.Close(); err != nil {
			log.Printf("Error closing reader for topic %s: %v", topic, err)
		} else {
			log.Printf("Closed reader for topic: %s", topic)
		}
	}

	// Wait for consumption goroutines to finish
	<-c.doneChan

	log.Println("Kafka consumer stopped successfully")
	return nil
}

// IsRunning returns true if the consumer is currently running
func (c *Consumer) IsRunning() bool {
	return c.running
}

// consumeFromTopic handles message consumption for a specific topic
func (c *Consumer) consumeFromTopic(ctx context.Context, topic string, reader *kafka.Reader) {
	log.Printf("Started consuming from topic: %s", topic)

	for {
		select {
		case <-c.stopChan:
			log.Printf("Stopping consumption from topic: %s", topic)
			return
		case <-ctx.Done():
			log.Printf("Context cancelled, stopping consumption from topic: %s", topic)
			return
		default:
			// Read message with timeout
			message, err := reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					log.Printf("Context cancelled while reading from topic: %s", topic)
					return
				}
				log.Printf("Error reading message from topic %s: %v", topic, err)
				time.Sleep(time.Second) // Back off on error
				continue
			}

			// Process the message (for now, just log it)
			c.processMessage(topic, message)

			// Commit the message
			if err := reader.CommitMessages(ctx, message); err != nil {
				log.Printf("Error committing message from topic %s: %v", topic, err)
			}
		}
	}
}

// processMessage handles the actual message processing
func (c *Consumer) processMessage(topic string, message kafka.Message) {
	log.Printf("Received event from topic '%s':", topic)
	log.Printf("  Partition: %d", message.Partition)
	log.Printf("  Offset: %d", message.Offset)
	log.Printf("  Key: %s", string(message.Key))
	log.Printf("  Value: %s", string(message.Value))
	log.Printf("  Headers: %v", message.Headers)
	log.Printf("  Time: %v", message.Time)
	log.Println("---")

	// TODO: Add your custom event processing logic here
	// For example:
	// - Parse the message value as JSON
	// - Route messages based on key or headers
	// - Store events in database
	// - Trigger workflows based on event type
}

// getStartOffset converts string offset setting to kafka offset constant
func (c *Consumer) getStartOffset() int64 {
	switch c.config.AutoOffsetReset {
	case "earliest":
		return kafka.FirstOffset
	case "latest":
		return kafka.LastOffset
	default:
		return kafka.LastOffset
	}
}
