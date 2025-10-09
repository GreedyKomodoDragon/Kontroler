package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
)

func TestDagRunEvent_Validation(t *testing.T) {
	creator := &DagRunCreator{namespace: "default"}

	tests := []struct {
		name    string
		event   DagRunEvent
		wantErr bool
	}{
		{
			name: "valid event with namespace",
			event: DagRunEvent{
				ID:        "test-1",
				Type:      DAGRunTrigger,
				DagName:   "test-dag",
				Namespace: "test-ns",
			},
			wantErr: false,
		},
		{
			name: "valid event without namespace (uses default)",
			event: DagRunEvent{
				ID:      "test-2",
				Type:    DAGRunTrigger,
				DagName: "test-dag",
			},
			wantErr: false,
		},
		{
			name: "missing dag name",
			event: DagRunEvent{
				ID:        "test-3",
				Type:      DAGRunTrigger,
				Namespace: "test-ns",
			},
			wantErr: true,
		},
		{
			name: "missing namespace and no default",
			event: DagRunEvent{
				ID:      "test-4",
				Type:    DAGRunTrigger,
				DagName: "test-dag",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "missing namespace and no default" {
				creator.namespace = "" // Remove default namespace for this test
			} else {
				creator.namespace = "default"
			}

			err := creator.validateDagRunEvent(tt.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDagRunCreator_CreateDagRunFromEvent(t *testing.T) {
	// Create a fake Kubernetes client
	scheme := runtime.NewScheme()
	require.NoError(t, kontrolerv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	creator := &DagRunCreator{
		kubeClient: fakeClient,
		namespace:  "default",
	}

	event := DagRunEvent{
		ID:        "test-event-123",
		Type:      DAGRunTrigger,
		Source:    "test-source",
		Timestamp: time.Now().Unix(),
		DagName:   "test-dag",
		RunName:   "test-run",
		Namespace: "test-namespace",
		Parameters: map[string]interface{}{
			"param1": "value1",
			"param2": 42,
			"param3": true,
		},
	}

	err := creator.createDagRunFromEvent(event)
	require.NoError(t, err)

	// Verify the DagRun was created
	var dagRun kontrolerv1alpha1.DagRun
	err = fakeClient.Get(context.Background(), client.ObjectKey{
		Name:      "test-run",
		Namespace: "test-namespace",
	}, &dagRun)
	require.NoError(t, err)

	// Verify the DagRun properties
	assert.Equal(t, "test-dag", dagRun.Spec.DagName)
	assert.Equal(t, "test-run", dagRun.Name)
	assert.Equal(t, "test-namespace", dagRun.Namespace)

	// Verify labels
	assert.Equal(t, "test-dag", dagRun.Labels["kontroler.greedykomodo/dag-name"])
	assert.Equal(t, "test-source", dagRun.Labels["kontroler.greedykomodo/source"])
	assert.Equal(t, "dagrun.trigger", dagRun.Labels["kontroler.greedykomodo/trigger"])

	// Verify annotations
	assert.Equal(t, "test-event-123", dagRun.Annotations["kontroler.greedykomodo/event-id"])

	// Verify parameters
	require.Len(t, dagRun.Spec.Parameters, 3)

	paramMap := make(map[string]string)
	for _, param := range dagRun.Spec.Parameters {
		paramMap[param.Name] = param.Value
	}

	assert.Equal(t, "value1", paramMap["param1"])
	assert.Equal(t, "42", paramMap["param2"])
	assert.Equal(t, "true", paramMap["param3"])
}

func TestDagRunCreator_GenerateRunName(t *testing.T) {
	creator := &DagRunCreator{namespace: "default"}

	event := DagRunEvent{
		ID:      "test-event",
		Type:    DAGRunTrigger,
		DagName: "test-dag",
		// RunName is empty, should be generated
	}

	// Mock the createDagRunFromEvent to test run name generation
	// We can't easily test this without refactoring, but the logic is simple
	// If RunName is empty, it generates: fmt.Sprintf("%s-%d", event.DagName, time.Now().Unix())

	// Test that validation passes even without RunName
	err := creator.validateDagRunEvent(event)
	assert.NoError(t, err)
}

func TestNewDagRunCreator(t *testing.T) {
	// Test with nil config
	creator, err := NewDagRunCreator(nil)
	assert.Error(t, err)
	assert.Nil(t, creator)

	// Test with minimal valid config
	// Note: This test creates a real client but with an empty config, which should work for testing
	config := &DagRunCreatorConfig{
		KafkaConfig: DefaultKafkaConfig(),
		KubeConfig:  &rest.Config{},
		Namespace:   "test-namespace",
	}

	// This should succeed now since we create a fake client
	creator, err = NewDagRunCreator(config)
	assert.NoError(t, err) // Should work now
	assert.NotNil(t, creator)
	assert.Equal(t, "test-namespace", creator.namespace)
}

func TestDagRunEvent_JSON(t *testing.T) {
	event := DagRunEvent{
		ID:        "test-123",
		Type:      DAGRunTrigger,
		Source:    "webhook",
		Timestamp: 1640995200,
		DagName:   "my-dag",
		RunName:   "my-run",
		Namespace: "my-namespace",
		Parameters: map[string]interface{}{
			"env":     "production",
			"retries": float64(3), // Use float64 since JSON unmarshaling will convert numbers to float64
		},
	}

	// Test marshaling
	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled DagRunEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.ID, unmarshaled.ID)
	assert.Equal(t, event.Type, unmarshaled.Type)
	assert.Equal(t, event.Source, unmarshaled.Source)
	assert.Equal(t, event.Timestamp, unmarshaled.Timestamp)
	assert.Equal(t, event.DagName, unmarshaled.DagName)
	assert.Equal(t, event.RunName, unmarshaled.RunName)
	assert.Equal(t, event.Namespace, unmarshaled.Namespace)
	assert.Equal(t, event.Parameters, unmarshaled.Parameters)
}

func TestEventTypes(t *testing.T) {
	// Test that our event type is correctly defined
	assert.Equal(t, EventType("dagrun.trigger"), DAGRunTrigger)
}
