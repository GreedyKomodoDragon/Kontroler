package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
)

type LogStore struct {
	StoreType  string                   `yaml:"storeType"`
	FileSystem FileSystemLogStoreConfig `yaml:"fileSystem"`
	S3Configs  S3LogStoreConfig         `yaml:"s3"`
}

type S3LogStoreConfig struct {
	BucketName string `yaml:"bucketName"`
	Endpoint   string `yaml:"endpoint,omitempty"`
}

type FileSystemLogStoreConfig struct {
	BaseDir string `yaml:"baseDir"`
}

type RateLimit struct {
	Enabled       bool   `yaml:"enabled"`
	MaxAttempts   int    `yaml:"maxAttempts"`
	BlockDuration string `yaml:"blockDuration"`
}

type ServerConfig struct {
	KubeConfigPath string    `yaml:"kubeConfigPath"`
	LogStorage     LogStore  `yaml:"logStorage"`
	RateLimit      RateLimit `yaml:"rateLimit"`
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
	if config.LogStorage.StoreType == "" {
		if bucket := os.Getenv("S3_BUCKETNAME"); bucket != "" {
			config.LogStorage.StoreType = "s3"
			config.LogStorage.S3Configs = S3LogStoreConfig{
				BucketName: bucket,
				Endpoint:   os.Getenv("S3_ENDPOINT"),
			}
		} else if dir := os.Getenv("LOG_DIR"); dir != "" {
			config.LogStorage.StoreType = "filesystem"
			config.LogStorage.FileSystem = FileSystemLogStoreConfig{
				BaseDir: dir,
			}
		}
		return // If we set type from env, we're done
	}

	// If type is specified but values are missing, try to fill from env
	switch config.LogStorage.StoreType {
	case "s3":
		// If bucket name is empty, try to get from env
		if config.LogStorage.S3Configs.BucketName == "" {
			config.LogStorage.S3Configs.BucketName = os.Getenv("S3_BUCKETNAME")
		}
		// If endpoint is empty, try to get from env
		if config.LogStorage.S3Configs.Endpoint == "" {
			config.LogStorage.S3Configs.Endpoint = os.Getenv("S3_ENDPOINT")
		}
	case "filesystem":
		if config.LogStorage.FileSystem.BaseDir == "" {
			config.LogStorage.FileSystem = FileSystemLogStoreConfig{
				BaseDir: os.Getenv("LOG_DIR"),
			}
		}
	}
}

// validateConfig ensures the configuration is valid
func validateConfig(config *ServerConfig) error {
	if config.LogStorage.StoreType == "" {
		return nil // No log storage configured is valid
	}

	switch config.LogStorage.StoreType {
	case "s3":
		if config.LogStorage.S3Configs.BucketName == "" {
			return fmt.Errorf("s3 bucket name is required for s3 log storage")
		}
	case "filesystem":
		if config.LogStorage.FileSystem.BaseDir == "" {
			return fmt.Errorf("directory path is required for filesystem log storage")
		}
	default:
		return fmt.Errorf("invalid log storage type: %s", config.LogStorage.StoreType)
	}

	return nil
}

func validatePaths(config *ServerConfig) error {
	if config.KubeConfigPath != "" {
		cleanPath := filepath.Clean(config.KubeConfigPath)
		if !filepath.IsAbs(cleanPath) {
			return fmt.Errorf("kubeConfigPath must be an absolute path, got: %s", config.KubeConfigPath)
		}
		config.KubeConfigPath = cleanPath
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

	// Set sensible defaults for rate limiting
	if config.RateLimit.MaxAttempts == 0 {
		config.RateLimit.MaxAttempts = 5
	}
	if config.RateLimit.BlockDuration == "" {
		config.RateLimit.BlockDuration = "15m"
	}

	// Apply environment-based configuration
	applyEnvironmentConfig(config)

	// Validate the final configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	if err := validatePaths(config); err != nil {
		return nil, err
	}

	return config, nil
}

func ParseDuration(durationStr string) (time.Duration, error) {
	if durationStr == "" {
		return 0, nil
	}
	return time.ParseDuration(durationStr)
}
