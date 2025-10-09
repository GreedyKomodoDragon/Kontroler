package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
)

// EventType represents the type of event
type EventType string

const (
	DAGRunTrigger EventType = "dagrun.trigger"
)

// DagRunEvent represents an event that should trigger a DAG run
type DagRunEvent struct {
	ID         string                 `json:"id"`
	Type       EventType              `json:"type"`
	Source     string                 `json:"source"`
	Timestamp  int64                  `json:"timestamp"`
	DagName    string                 `json:"dagName"`
	RunName    string                 `json:"runName,omitempty"` // Optional, will be generated if empty
	Namespace  string                 `json:"namespace"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// DagRunCreator handles creating DagRuns from Kafka events
type DagRunCreator struct {
	consumer   *Consumer
	kubeClient client.Client
	namespace  string // Default namespace if not specified in event
}

// DagRunCreatorConfig extends KafkaConfig with DagRun creation specific settings
type DagRunCreatorConfig struct {
	*KafkaConfig
	KubeConfig *rest.Config
	Namespace  string // Default namespace for DagRuns
}

// NewDagRunCreator creates a new Kafka consumer that creates DagRuns from events
func NewDagRunCreator(config *DagRunCreatorConfig) (*DagRunCreator, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.KafkaConfig == nil {
		config.KafkaConfig = DefaultKafkaConfig()
	}

	if config.Namespace == "" {
		config.Namespace = "default"
	}

	// Create scheme for the Kubernetes client
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kontrolerv1alpha1.AddToScheme(scheme))

	// Create Kubernetes client
	kubeClient, err := client.New(config.KubeConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	consumer := NewConsumer(config.KafkaConfig)
	return &DagRunCreator{
		consumer:   consumer,
		kubeClient: kubeClient,
		namespace:  config.Namespace,
	}, nil
}

// processMessage overrides the default message processing to create DagRuns
func (d *DagRunCreator) processMessage(topic string, message kafka.Message) {
	log.Printf("Received event from topic '%s' for DAG run creation", topic)

	// Try to parse the message as a DagRunEvent
	var event DagRunEvent
	if err := json.Unmarshal(message.Value, &event); err != nil {
		log.Printf("Failed to parse message as DagRunEvent: %v", err)
		log.Printf("Message content: %s", string(message.Value))
		return
	}

	// Validate the event
	if err := d.validateDagRunEvent(event); err != nil {
		log.Printf("Invalid DagRunEvent: %v", err)
		return
	}

	// Create DagRun based on event type
	switch event.Type {
	case DAGRunTrigger:
		if err := d.createDagRunFromEvent(event); err != nil {
			log.Printf("Failed to create DagRun from event: %v", err)
			return
		}
		log.Printf("Successfully created DagRun '%s' for DAG '%s'", event.RunName, event.DagName)
	default:
		log.Printf("Unknown event type '%s', ignoring", event.Type)
	}
}

// validateDagRunEvent validates that the event has required fields
func (d *DagRunCreator) validateDagRunEvent(event DagRunEvent) error {
	if event.DagName == "" {
		return fmt.Errorf("dagName is required")
	}

	if event.Namespace == "" && d.namespace == "" {
		return fmt.Errorf("namespace must be specified in event or config")
	}

	return nil
}

// createDagRunFromEvent creates a Kubernetes DagRun resource from the event
func (d *DagRunCreator) createDagRunFromEvent(event DagRunEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use event namespace or fall back to default
	namespace := event.Namespace
	if namespace == "" {
		namespace = d.namespace
	}

	// Generate run name if not provided
	runName := event.RunName
	if runName == "" {
		runName = fmt.Sprintf("%s-%d", event.DagName, time.Now().Unix())
	}

	// Convert parameters to ParameterSpec format
	var parameters []kontrolerv1alpha1.ParameterSpec
	for name, value := range event.Parameters {
		// Convert the interface{} value to string
		valueStr := fmt.Sprintf("%v", value)
		parameters = append(parameters, kontrolerv1alpha1.ParameterSpec{
			Name:  name,
			Value: valueStr,
		})
	}

	// Create the DagRun resource
	dagRun := &kontrolerv1alpha1.DagRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      runName,
			Namespace: namespace,
			Labels: map[string]string{
				"kontroler.greedykomodo/dag-name": event.DagName,
				"kontroler.greedykomodo/source":   event.Source,
				"kontroler.greedykomodo/trigger":  string(event.Type),
			},
			Annotations: map[string]string{
				"kontroler.greedykomodo/event-id":        event.ID,
				"kontroler.greedykomodo/event-timestamp": fmt.Sprintf("%d", event.Timestamp),
			},
		},
		Spec: kontrolerv1alpha1.DagRunSpec{
			DagName:    event.DagName,
			Parameters: parameters,
		},
	}

	// Create the DagRun in Kubernetes
	if err := d.kubeClient.Create(ctx, dagRun); err != nil {
		return fmt.Errorf("failed to create DagRun in Kubernetes: %w", err)
	}

	log.Printf("Created DagRun: %s/%s for DAG: %s", namespace, runName, event.DagName)
	log.Printf("  Event ID: %s", event.ID)
	log.Printf("  Source: %s", event.Source)
	log.Printf("  Parameters: %+v", event.Parameters)

	return nil
}

// Subscribe wraps the consumer's Subscribe method
func (d *DagRunCreator) Subscribe(topics ...string) error {
	return d.consumer.Subscribe(topics...)
}

// Stop wraps the consumer's Stop method
func (d *DagRunCreator) Stop() error {
	return d.consumer.Stop()
}

// IsRunning wraps the consumer's IsRunning method
func (d *DagRunCreator) IsRunning() bool {
	return d.consumer.IsRunning()
}

// Start starts the DagRun creator with custom message processing
func (d *DagRunCreator) Start(ctx context.Context) error {
	log.Println("Starting DagRun creator...")

	if d.consumer.running {
		return fmt.Errorf("consumer is already running")
	}

	if len(d.consumer.readers) == 0 {
		return fmt.Errorf("no topics subscribed")
	}

	d.consumer.running = true

	// Start a goroutine for each topic reader with our custom processing
	for topic, reader := range d.consumer.readers {
		go d.consumeFromTopicWithDagRunCreation(ctx, topic, reader)
	}

	log.Printf("DagRun creator started, consuming from %d topics", len(d.consumer.readers))

	// Wait for stop signal or context cancellation
	select {
	case <-ctx.Done():
		log.Println("Context cancelled, stopping DagRun creator")
		return d.Stop()
	case <-d.consumer.stopChan:
		log.Println("Stop signal received")
	}

	close(d.consumer.doneChan)
	return nil
}

// consumeFromTopicWithDagRunCreation handles message consumption with DAG run creation
func (d *DagRunCreator) consumeFromTopicWithDagRunCreation(ctx context.Context, topic string, reader *kafka.Reader) {
	log.Printf("Started consuming from topic: %s (with DAG run creation)", topic)

	for {
		select {
		case <-d.consumer.stopChan:
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

			// Process the message with our custom DAG run creation logic
			d.processMessage(topic, message)

			// Commit the message
			if err := reader.CommitMessages(ctx, message); err != nil {
				log.Printf("Error committing message from topic %s: %v", topic, err)
			}
		}
	}
}
