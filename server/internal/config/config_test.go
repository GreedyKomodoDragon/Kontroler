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
		name           string
		yamlContent    string
		envVars        map[string]string
		wantErr        bool
		wantLogType    string
		wantS3Bucket   string
		wantS3Endpoint string
		wantFSDir      string
	}{
		{
			name: "valid filesystem config from yaml",
			yamlContent: `
logStorage:
  storeType: "filesystem"
  fileSystem:
    baseDir: /var/log/kontroler
`,
			wantLogType: "filesystem",
			wantFSDir:   "/var/log/kontroler",
		},
		{
			name: "valid s3 config from yaml",
			yamlContent: `
logStorage:
  storeType: "s3"
  s3:
    bucketName: my-log-bucket
    endpoint: https://my-custom-s3.example.com
`,
			wantLogType:    "s3",
			wantS3Bucket:   "my-log-bucket",
			wantS3Endpoint: "https://my-custom-s3.example.com",
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
				"S3_ENDPOINT":   "https://env-s3.example.com",
			},
			wantLogType:    "s3",
			wantS3Bucket:   "env-bucket",
			wantS3Endpoint: "https://env-s3.example.com",
		},
		{
			name: "invalid storage type",
			yamlContent: `
logStorage:
  storeType: "invalid"
  s3:
	bucketName: my-log-bucket
`,
			wantErr: true,
		},
		{
			name: "missing s3 bucket name",
			yamlContent: `
logStorage:
  storeType: "s3"
`,
			wantErr: true,
		},
		{
			name: "missing filesystem directory",
			yamlContent: `
logStorage:
  storeType: filesystem
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
  storeType: "filesystem"
  fileSystem:
    baseDir: /yaml/path
`,
			envVars: map[string]string{
				"S3_BUCKETNAME": "env-bucket",
				"LOG_DIR":       "/env/path",
			},
			wantLogType: "filesystem",
			wantFSDir:   "/yaml/path",
		},
		{
			name: "s3 yaml with env endpoint",
			yamlContent: `
logStorage:
  storeType: "s3"
  s3:
    bucketName: yaml-bucket
`,
			envVars: map[string]string{
				"S3_ENDPOINT": "https://env-s3.example.com",
			},
			wantLogType:    "s3",
			wantS3Bucket:   "yaml-bucket",
			wantS3Endpoint: "https://env-s3.example.com",
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
				t.Logf("Got config: type=%q, s3bucket=%q, s3endpoint=%q, fsdir=%q",
					cfg.LogStorage.StoreType,
					cfg.LogStorage.S3Configs.BucketName,
					cfg.LogStorage.S3Configs.Endpoint,
					cfg.LogStorage.FileSystem.BaseDir)
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
				require.Equal(t, tc.wantLogType, cfg.LogStorage.StoreType, "Wrong storage type")
			}
			if tc.wantS3Bucket != "" {
				require.Equal(t, tc.wantS3Bucket, cfg.LogStorage.S3Configs.BucketName, "Wrong S3 bucket")
			}
			if tc.wantS3Endpoint != "" {
				require.Equal(t, tc.wantS3Endpoint, cfg.LogStorage.S3Configs.Endpoint, "Wrong S3 endpoint")
			}
			if tc.wantFSDir != "" {
				require.Equal(t, tc.wantFSDir, cfg.LogStorage.FileSystem.BaseDir, "Wrong filesystem dir")
			}
		})
	}
}

func TestParseConfigPaths(t *testing.T) {
	// Test valid absolute path
	t.Run("valid absolute path", func(t *testing.T) {
		tmpfile := createTempConfigFile(t, `
kubeConfigPath: /absolute/path/to/kubeconfig
`)
		defer os.Remove(tmpfile)

		config, err := ParseConfig(tmpfile)
		require.NoError(t, err)
		require.True(t, filepath.IsAbs(config.KubeConfigPath))
	})

	// Test invalid relative path
	t.Run("invalid relative path", func(t *testing.T) {
		tmpfile := createTempConfigFile(t, `
kubeConfigPath: relative/path/to/kubeconfig
`)
		defer os.Remove(tmpfile)

		_, err := ParseConfig(tmpfile)
		require.Error(t, err)
		require.Contains(t, err.Error(), "kubeConfigPath must be an absolute path")
	})

	// Test empty path
	t.Run("empty path", func(t *testing.T) {
		tmpfile := createTempConfigFile(t, `
kubeConfigPath: ""
`)
		defer os.Remove(tmpfile)

		config, err := ParseConfig(tmpfile)
		require.NoError(t, err)
		require.Empty(t, config.KubeConfigPath)
	})
}

func createTempConfigFile(t *testing.T, content string) string {
	tmpfile, err := os.CreateTemp("", "config*.yaml")
	require.NoError(t, err)

	_, err = tmpfile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())

	return tmpfile.Name()
}
