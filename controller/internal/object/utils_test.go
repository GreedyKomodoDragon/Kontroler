package object

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestRemoveFinalizer_RetryOnConflict(t *testing.T) {
	podName := "test-pod"
	ns := "default"
	finalizer := "kontroler/logcollection"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:       podName,
			Namespace:  ns,
			Finalizers: []string{finalizer},
		},
	}

	client := fake.NewSimpleClientset(pod)

	// Simulate a conflict on the first Update, then succeed returning the updated object
	attempt := 0
	client.PrependReactor("update", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		ua := action.(k8stesting.UpdateAction)
		obj := ua.GetObject().(*corev1.Pod)
		if attempt == 0 {
			attempt++
			return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, obj.Name, fmt.Errorf("conflict"))
		}
		// On subsequent attempt return the updated object
		return true, obj, nil
	})

	if err := RemoveFinalizer(client, podName, ns, finalizer); err != nil {
		t.Fatalf("RemoveFinalizer returned error: %v", err)
	}
}

func TestRemoveFinalizer_PodNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()
	// No pod created in fake client; RemoveFinalizer should treat NotFound as success
	if err := RemoveFinalizer(client, "does-not-exist", "default", "kontroler/logcollection"); err != nil {
		t.Fatalf("expected nil when pod not found, got: %v", err)
	}
}
