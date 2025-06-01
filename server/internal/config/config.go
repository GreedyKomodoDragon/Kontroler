package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type LogStorageConfig struct {
	Type          string `yaml:"type"`          // "s3" or "filesystem"
	S3BucketName  string `yaml:"s3BucketName"`  // required for s3
	FileSystemDir string `yaml:"fileSystemDir"` // required for filesystem
}

type ServerConfig struct {
	KubeConfigPath string           `yaml:"kubeConfigPath"`
	LogStorage     LogStorageConfig `yaml:"logStorage"`
}

// loadConfigFromYAML attempts to load configuration from a YAML file
func loadConfigFromYAML(configPath string) (*ServerConfig, error) {
	config := &ServerConfig{}

	if configPath == "" {
		return config, nil
	}

	yamlFile, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return config, nil // File doesn't exist, return empty config
	} else if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err = yaml.Unmarshal(yamlFile, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// applyEnvironmentConfig applies any environment-based configuration
func applyEnvironmentConfig(config *ServerConfig) {
	// Check environment variables if YAML didn't specify a type
	if config.LogStorage.Type == "" {
		if bucket := os.Getenv("S3_BUCKETNAME"); bucket != "" {
			config.LogStorage.Type = "s3"
			config.LogStorage.S3BucketName = bucket
		} else if dir := os.Getenv("LOG_DIR"); dir != "" {
			config.LogStorage.Type = "filesystem"
			config.LogStorage.FileSystemDir = dir
		}
		return // If we set type from env, we're done
	}

	// If type is specified but values are missing, try to fill from env
	switch config.LogStorage.Type {
	case "s3":
		if config.LogStorage.S3BucketName == "" {
			config.LogStorage.S3BucketName = os.Getenv("S3_BUCKETNAME")
		}
	case "filesystem":
		if config.LogStorage.FileSystemDir == "" {
			config.LogStorage.FileSystemDir = os.Getenv("LOG_DIR")
		}
	}
}

// validateConfig ensures the configuration is valid
func validateConfig(config *ServerConfig) error {
	if config.LogStorage.Type == "" {
		return nil // No log storage configured is valid
	}

	switch config.LogStorage.Type {
	case "s3":
		if config.LogStorage.S3BucketName == "" {
			return fmt.Errorf("s3 bucket name is required for s3 log storage")
		}
	case "filesystem":
		if config.LogStorage.FileSystemDir == "" {
			return fmt.Errorf("directory path is required for filesystem log storage")
		}
	default:
		return fmt.Errorf("invalid log storage type: %s", config.LogStorage.Type)
	}

	return nil
}

func ParseConfig(configPath string) (*ServerConfig, error) {
	var config *ServerConfig
	var err error

	if configPath != "" {
		// Try to load from YAML only if path is provided
		config, err = loadConfigFromYAML(configPath)
		if err != nil {
			return nil, err
		}
	} else {
		// No config path provided, start with empty config
		config = &ServerConfig{}
	}

	// Apply environment-based configuration
	applyEnvironmentConfig(config)

	// Validate the final configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}
