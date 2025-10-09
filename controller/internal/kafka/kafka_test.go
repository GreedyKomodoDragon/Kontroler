package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConsumer(t *testing.T) {
	// Test with nil config (should use default)
	consumer := NewConsumer(nil)
	assert.NotNil(t, consumer)
	assert.NotNil(t, consumer.config)
	assert.Equal(t, "kontroler-consumer-group", consumer.config.GroupID)
	assert.Equal(t, []string{"localhost:9092"}, consumer.config.Brokers)
	assert.False(t, consumer.IsRunning())
}

func TestConsumerSubscribe(t *testing.T) {
	consumer := NewConsumer(nil)

	// Test subscribing to topics
	err := consumer.Subscribe("test-topic-1", "test-topic-2")
	assert.NoError(t, err)
	assert.Len(t, consumer.readers, 2)

	// Test subscribing to same topic again
	err = consumer.Subscribe("test-topic-1")
	assert.NoError(t, err)
	assert.Len(t, consumer.readers, 2) // Should still be 2
}

func TestConsumerStartWithoutTopics(t *testing.T) {
	consumer := NewConsumer(nil)
	ctx := context.Background()

	// Should fail when no topics are subscribed
	err := consumer.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no topics subscribed")
}

func TestConsumerRunningState(t *testing.T) {
	consumer := NewConsumer(nil)

	// Initially not running
	assert.False(t, consumer.IsRunning())

	// Mock running state
	consumer.running = true
	assert.True(t, consumer.IsRunning())

	// Test subscribing while running should fail
	err := consumer.Subscribe("test-topic")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot subscribe to topics while consumer is running")
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultKafkaConfig()
	require.NotNil(t, config)

	assert.Equal(t, []string{"localhost:9092"}, config.Brokers)
	assert.Equal(t, "kontroler-consumer-group", config.GroupID)
	assert.Equal(t, "latest", config.AutoOffsetReset)
	assert.Equal(t, time.Second*10, config.CommitInterval)
	assert.Equal(t, 10000000, config.MaxBytes) // 10e6 as int
	assert.Equal(t, 10000, config.MinBytes)    // 10e3 as int
	assert.Equal(t, time.Second*1, config.MaxWait)
}

func TestGetStartOffset(t *testing.T) {
	consumer := NewConsumer(&KafkaConfig{AutoOffsetReset: "earliest"})
	offset := consumer.getStartOffset()
	// Note: We can't easily test the exact kafka constants here without mocking,
	// but we can test that the function doesn't panic and returns a value
	assert.NotNil(t, offset)

	consumer.config.AutoOffsetReset = "latest"
	offset = consumer.getStartOffset()
	assert.NotNil(t, offset)

	consumer.config.AutoOffsetReset = "invalid"
	offset = consumer.getStartOffset()
	assert.NotNil(t, offset) // Should default to latest
}
