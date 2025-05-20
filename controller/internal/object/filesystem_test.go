package object

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewFileSystemLogStore(t *testing.T) {
	tests := []struct {
		name    string
		envPath string
		wantErr bool
	}{
		{
			name:    "default path",
			envPath: "",
			wantErr: false,
		},
		{
			name:    "custom path",
			envPath: "/tmp/custom-logs",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewFileSystemLogStore(tt.envPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFileSystemLogStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if store == nil {
				t.Error("NewFileSystemLogStore() returned nil store")
				return
			}

			fs, ok := store.(*fileSystemLogStore)
			if !ok {
				t.Error("NewFileSystemLogStore() did not return a fileSystemLogStore")
				return
			}

			expectedPath := tt.envPath
			if expectedPath == "" {
				expectedPath = "/tmp/kontroler-logs"
			}
			if fs.baseDir != expectedPath {
				t.Errorf("baseDir = %v, want %v", fs.baseDir, expectedPath)
			}

			// Clean up
			if tt.envPath != "" {
				os.RemoveAll(tt.envPath)
			}
		})
	}
}

func TestFetchingStatus(t *testing.T) {
	store, err := NewFileSystemLogStore(t.TempDir())
	require.NoError(t, err, "Failed to create store")

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}
	dagRunId := 123

	// Initial state should be not fetching
	require.False(t, store.IsFetching(dagRunId, pod), "Should not be fetching initially")

	// Mark as fetching
	require.NoError(t, store.MarkAsFetching(dagRunId, pod), "Should be able to mark as fetching")

	// Should be fetching now
	require.True(t, store.IsFetching(dagRunId, pod), "Should be fetching after marking")

	// Try marking again - should fail
	require.Error(t, store.MarkAsFetching(dagRunId, pod), "Should not be able to mark as fetching twice")

	// Unlist fetching
	store.UnlistFetching(dagRunId, pod)

	// Should not be fetching anymore
	require.False(t, store.IsFetching(dagRunId, pod), "Should not be fetching after unlisting")
}

func TestDeleteLogs(t *testing.T) {
	store, err := NewFileSystemLogStore(t.TempDir())
	require.NoError(t, err, "Failed to create store")

	fs := store.(*fileSystemLogStore)
	dagrunId := 456

	// Create a test directory with some content
	logDir := filepath.Join(fs.baseDir, "456")
	require.NoError(t, os.MkdirAll(logDir, 0755), "Failed to create test directory")

	testFile := filepath.Join(logDir, "test.log")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644), "Failed to create test file")

	// Test deletion
	ctx := context.Background()
	require.NoError(t, store.DeleteLogs(ctx, dagrunId), "DeleteLogs should not return an error")

	// Verify directory is gone
	_, err = os.Stat(logDir)
	require.Error(t, err, "Log directory should be deleted")

	// Test deleting non-existent directory
	require.NoError(t, store.DeleteLogs(ctx, 789), "DeleteLogs should not fail for non-existent directory")
}
