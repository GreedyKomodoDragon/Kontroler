package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type ControllerConfig struct {
	KubeConfigPath   string        `yaml:"kubeConfigPath"`
	LeaderElectionID string        `yaml:"leaderElectionID"`
	Workers          WorkerConfigs `yaml:"workers"`
	LogStore         LogStore      `yaml:"logStore"`
}

type LogStore struct {
	StoreType  string                   `yaml:"storeType"`
	FileSystem FileSystemLogStoreConfig `yaml:"fileSystem"`
}

type FileSystemLogStoreConfig struct {
	BaseDir string `yaml:"baseDir"`
}

type WorkerConfigs struct {
	WorkerType   string         `yaml:"workerType"` // "memory" or "pebble"
	QueueDir     string         `yaml:"queueDir"`   // directory for pebble queue storage
	Workers      []WorkerConfig `yaml:"workers"`
	PollDuration string         `yaml:"pollDuration"`
}

type WorkerConfig struct {
	Namespace string `yaml:"namespace"`
	Count     int    `yaml:"count"`
}

func ParseConfig(configPath string) (*ControllerConfig, error) {
	cConfig := &ControllerConfig{}

	// parse config file at configPath
	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(yamlFile, cConfig); err != nil {
		return nil, err
	}

	// validate worker configs
	if len(cConfig.Workers.Workers) == 0 {
		return nil, fmt.Errorf("missing worker configs, must provide at least one worker config")
	}

	// validate worker type and queue directory
	switch cConfig.Workers.WorkerType {
	case "memory", "pebble":
		if cConfig.Workers.WorkerType == "pebble" && cConfig.Workers.QueueDir == "" {
			return nil, fmt.Errorf("queueDir must be specified when using pebble worker type")
		}
	default:
		return nil, fmt.Errorf("invalid worker type %q, must be 'memory' or 'pebble'", cConfig.Workers.WorkerType)
	}

	// Ensure directory exists for pebble
	if cConfig.Workers.WorkerType == "pebble" {
		if err := os.MkdirAll(cConfig.Workers.QueueDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create queue directory: %w", err)
		}
	}

	// Parse and validate poll duration
	if cConfig.Workers.PollDuration == "" {
		cConfig.Workers.PollDuration = "100ms"
	}

	// leaderElectionID
	if leaderElectionID := os.Getenv("LEADER_ELECTION_ID"); leaderElectionID != "" {
		cConfig.LeaderElectionID = leaderElectionID
	} else if cConfig.LeaderElectionID == "" {
		return nil, fmt.Errorf("missing LEADER_ELECTION_ID, must provide LEADER_ELECTION_ID")
	}

	// logstore
	if cConfig.LogStore.StoreType == "" {
		cConfig.LogStore.StoreType = "filesystem"
	}

	return cConfig, nil
}
