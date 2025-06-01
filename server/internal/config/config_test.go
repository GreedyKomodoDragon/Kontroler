package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config_test")
	require.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name         string
		yamlContent  string
		envVars      map[string]string
		wantErr      bool
		wantLogType  string
		wantS3Bucket string
		wantFSDir    string
	}{
		{
			name: "valid filesystem config from yaml",
			yamlContent: `
logStorage:
  type: filesystem
  fileSystemDir: /var/log/kontroler
`,
			wantLogType: "filesystem",
			wantFSDir:   "/var/log/kontroler",
		},
		{
			name: "valid s3 config from yaml",
			yamlContent: `
logStorage:
  type: s3
  s3BucketName: my-log-bucket
`,
			wantLogType:  "s3",
			wantS3Bucket: "my-log-bucket",
		},
		{
			name: "filesystem config from env",
			envVars: map[string]string{
				"LOG_DIR": "/mnt/logs",
			},
			wantLogType: "filesystem",
			wantFSDir:   "/mnt/logs",
		},
		{
			name: "s3 config from env",
			envVars: map[string]string{
				"S3_BUCKETNAME": "env-bucket",
			},
			wantLogType:  "s3",
			wantS3Bucket: "env-bucket",
		},
		{
			name: "invalid storage type",
			yamlContent: `
logStorage:
  type: invalid
  fileSystemDir: /var/log/kontroler
`,
			wantErr: true,
		},
		{
			name: "missing s3 bucket name",
			yamlContent: `
logStorage:
  type: s3
`,
			wantErr: true,
		},
		{
			name: "missing filesystem directory",
			yamlContent: `
logStorage:
  type: filesystem
`,
			wantErr: true,
		},
		{
			name: "no config is valid",
			yamlContent: `
kubeConfigPath: /some/path
`,
		},
		{
			name: "yaml takes precedence over env",
			yamlContent: `
logStorage:
  type: filesystem
  fileSystemDir: /yaml/path
`,
			envVars: map[string]string{
				"S3_BUCKETNAME": "env-bucket",
				"LOG_DIR":       "/env/path",
			},
			wantLogType: "filesystem",
			wantFSDir:   "/yaml/path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new test file for each test
			configPath := filepath.Join(tempDir, fmt.Sprintf("%s.yaml", tc.name))

			// Write test config file if content provided
			if tc.yamlContent != "" {
				err := os.WriteFile(configPath, []byte(tc.yamlContent), 0644)
				require.NoError(t, err, "Failed to write test config")
			} else {
				configPath = "" // No YAML content means no config file
			}

			// Clear existing environment variables
			os.Unsetenv("S3_BUCKETNAME")
			os.Unsetenv("LOG_DIR")

			// Set test environment variables
			for k, v := range tc.envVars {
				t.Logf("Setting env var %s=%s", k, v)
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Parse config and log inputs/outputs for debugging
			t.Logf("Testing with configPath=%q and env vars=%v", configPath, tc.envVars)
			cfg, err := ParseConfig(configPath)
			if cfg != nil {
				t.Logf("Got config: type=%q, s3bucket=%q, fsdir=%q",
					cfg.LogStorage.Type,
					cfg.LogStorage.S3BucketName,
					cfg.LogStorage.FileSystemDir)
			}

			// Check error
			if tc.wantErr {
				require.Error(t, err, "Expected error but got none")
				return
			}
			require.NoError(t, err, "Unexpected error")
			require.NotNil(t, cfg, "Config should not be nil")

			// Check results
			if tc.wantLogType != "" {
				require.Equal(t, tc.wantLogType, cfg.LogStorage.Type, "Wrong storage type")
			}
			if tc.wantS3Bucket != "" {
				require.Equal(t, tc.wantS3Bucket, cfg.LogStorage.S3BucketName, "Wrong S3 bucket")
			}
			if tc.wantFSDir != "" {
				require.Equal(t, tc.wantFSDir, cfg.LogStorage.FileSystemDir, "Wrong filesystem dir")
			}
		})
	}
}
