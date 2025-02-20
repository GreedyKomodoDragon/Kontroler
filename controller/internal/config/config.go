package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type ControllerConfig struct {
	KubeConfigPath   string   `yaml:"kubeConfigPath"`
	Namespaces       []string `yaml:"namespaces"`
	LeaderElectionID string   `yaml:"leaderElectionID"`
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

	// env overrides

	// namespaces
	if namespaces := os.Getenv("NAMESPACES"); namespaces != "" {
		cConfig.Namespaces = strings.Split(namespaces, ",")
	} else if cConfig.Namespaces == nil {
		cConfig.Namespaces = []string{"default"}
	}

	// leaderElectionID
	if leaderElectionID := os.Getenv("LEADER_ELECTION_ID"); leaderElectionID != "" {
		cConfig.LeaderElectionID = leaderElectionID
	} else if cConfig.LeaderElectionID == "" {
		return nil, fmt.Errorf("missing LEADER_ELECTION_ID, must provide LEADER_ELECTION_ID")
	}

	return cConfig, nil
}
