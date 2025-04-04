package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type ControllerConfig struct {
	KubeConfigPath   string        `yaml:"kubeConfigPath"`
	Namespaces       []string      `yaml:"namespaces"`
	LeaderElectionID string        `yaml:"leaderElectionID"`
	Workers          WorkerConfigs `yaml:"workers"`
}

type WorkerConfigs struct {
	WorkerType string         `yaml:"workerType"`
	Workers    []WorkerConfig `yaml:"workers"`
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

	// validate worker type - only memory is supported
	if cConfig.Workers.WorkerType != "memory" {
		return nil, fmt.Errorf("missing worker type, must provide worker type")
	}

	// env overrides

	// // namespaces
	// if namespaces := os.Getenv("NAMESPACES"); namespaces != "" {
	// 	cConfig.Namespaces = strings.Split(namespaces, ",")
	// } else if cConfig.Namespaces == nil {
	// 	cConfig.Namespaces = []string{"default"}
	// }

	// leaderElectionID
	if leaderElectionID := os.Getenv("LEADER_ELECTION_ID"); leaderElectionID != "" {
		cConfig.LeaderElectionID = leaderElectionID
	} else if cConfig.LeaderElectionID == "" {
		return nil, fmt.Errorf("missing LEADER_ELECTION_ID, must provide LEADER_ELECTION_ID")
	}

	return cConfig, nil
}
