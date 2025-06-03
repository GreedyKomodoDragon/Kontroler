package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		configYaml  string
		envVars     map[string]string
		expectError bool
		validate    func(*testing.T, *ControllerConfig)
	}{
		{
			name: "valid memory config",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
leaderElectionID: "test-controller"
workers:
  workerType: "memory"
  pollDuration: "200ms"
  workers:
    - namespace: "default"
      count: 2
logStorage:
  storeType: "s3"
  s3:
    bucketName: "my-test-bucket"
    endpoint: "https://minio.example.com:9000"
`,
			validate: func(t *testing.T, cfg *ControllerConfig) {
				assert.Equal(t, "/path/to/kube/config", cfg.KubeConfigPath)
				assert.Equal(t, "test-controller", cfg.LeaderElectionID)
				assert.Equal(t, "memory", cfg.Workers.WorkerType)
				assert.Equal(t, "200ms", cfg.Workers.PollDuration)
				assert.Len(t, cfg.Workers.Workers, 1)
				assert.Equal(t, "default", cfg.Workers.Workers[0].Namespace)
				assert.Equal(t, 2, cfg.Workers.Workers[0].Count)
				assert.Equal(t, "s3", cfg.LogStore.StoreType)
				assert.Equal(t, "my-test-bucket", cfg.LogStore.S3Configs.BucketName)
				assert.Equal(t, "https://minio.example.com:9000", cfg.LogStore.S3Configs.Endpoint)
			},
		},
		{
			name: "valid pebble config",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
leaderElectionID: "test-controller"
workers:
  workerType: "pebble"
  queueDir: "/tmp/test-queue"
  workers:
    - namespace: "default"
      count: 1
`,
			envVars: map[string]string{
				"LOG_DIR": "/var/log/default",
			},
			validate: func(t *testing.T, cfg *ControllerConfig) {
				assert.Equal(t, "pebble", cfg.Workers.WorkerType)
				assert.Equal(t, "/tmp/test-queue", cfg.Workers.QueueDir)
				assert.Len(t, cfg.Workers.Workers, 1)
				assert.Equal(t, "default", cfg.Workers.Workers[0].Namespace)
				assert.Equal(t, 1, cfg.Workers.Workers[0].Count)
				assert.Equal(t, "filesystem", cfg.LogStore.StoreType)
				assert.Equal(t, "/var/log/default", cfg.LogStore.FileSystem.BaseDir)
			},
		},
		{
			name: "missing leader election ID with env var",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
workers:
  workerType: "memory"
  workers:
    - namespace: "default"
      count: 1
`,
			envVars: map[string]string{
				"LEADER_ELECTION_ID": "env-controller",
				"LOG_DIR":            "/var/log/test",
			},
			validate: func(t *testing.T, cfg *ControllerConfig) {
				assert.Equal(t, "env-controller", cfg.LeaderElectionID)
				assert.Equal(t, "filesystem", cfg.LogStore.StoreType)
				assert.Equal(t, "/var/log/test", cfg.LogStore.FileSystem.BaseDir)
			},
		},
		{
			name: "invalid worker type",
			configYaml: `
workers:
  workerType: "invalid"
  workers:
    - namespace: "default"
      count: 1
`,
			expectError: true,
		},
		{
			name: "missing queue dir for pebble",
			configYaml: `
workers:
  workerType: "pebble"
  workers:
    - namespace: "default"
      count: 1
`,
			expectError: true,
		},
		{
			name: "missing worker configs",
			configYaml: `
workers:
  workerType: "memory"
`,
			expectError: true,
		},
		{
			name: "valid filesystem log config",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
leaderElectionID: "test-controller"
workers:
  workerType: "memory"
  workers:
    - namespace: "default"
      count: 1
logStorage:
  storeType: "filesystem"
  fileSystem:
    baseDir: "/var/log/kontroler"
`,
			validate: func(t *testing.T, cfg *ControllerConfig) {
				assert.Equal(t, "filesystem", cfg.LogStore.StoreType)
				assert.Equal(t, "/var/log/kontroler", cfg.LogStore.FileSystem.BaseDir)
			},
		},
		{
			name: "valid s3 log config without endpoint",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
leaderElectionID: "test-controller"
workers:
  workerType: "memory"
  workers:
    - namespace: "default"
      count: 1
logStorage:
  storeType: "s3"
  s3:
    bucketName: "my-test-bucket"
`,
			validate: func(t *testing.T, cfg *ControllerConfig) {
				assert.Equal(t, "s3", cfg.LogStore.StoreType)
				assert.Equal(t, "my-test-bucket", cfg.LogStore.S3Configs.BucketName)
				assert.Empty(t, cfg.LogStore.S3Configs.Endpoint)
			},
		},
		{
			name: "missing s3 bucket name",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
leaderElectionID: "test-controller"
workers:
  workerType: "memory"
  workers:
    - namespace: "default"
      count: 1
logStorage:
  storeType: "s3"
`,
			expectError: true,
		},
		{
			name: "missing filesystem base dir",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
leaderElectionID: "test-controller"
workers:
  workerType: "memory"
  workers:
    - namespace: "default"
      count: 1
logStorage:
  storeType: "filesystem"
`,
			expectError: true,
		},
		{
			name: "invalid log storage type",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
leaderElectionID: "test-controller"
workers:
  workerType: "memory"
  workers:
    - namespace: "default"
      count: 1
logStorage:
  storeType: "invalid"
`,
			expectError: true,
		},
		{
			name: "default filesystem with LOG_DIR",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
leaderElectionID: "test-controller"
workers:
  workerType: "memory"
  workers:
    - namespace: "default"
      count: 1
`,
			envVars: map[string]string{
				"LOG_DIR": "/var/log/env",
			},
			validate: func(t *testing.T, cfg *ControllerConfig) {
				assert.Equal(t, "filesystem", cfg.LogStore.StoreType)
				assert.Equal(t, "/var/log/env", cfg.LogStore.FileSystem.BaseDir)
			},
		},
		{
			name: "default filesystem without LOG_DIR",
			configYaml: `
kubeConfigPath: "/path/to/kube/config"
leaderElectionID: "test-controller"
workers:
  workerType: "memory"
  workers:
    - namespace: "default"
      count: 1
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			err := os.WriteFile(configPath, []byte(tt.configYaml), 0644)
			require.NoError(t, err)

			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Parse config
			cfg, err := ParseConfig(configPath)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.validate != nil {
				tt.validate(t, cfg)
			}

			// Clean up any created directories
			if cfg != nil && cfg.Workers.WorkerType == "pebble" && cfg.Workers.QueueDir != "" {
				_ = os.RemoveAll(cfg.Workers.QueueDir)
			}
		})
	}
}
