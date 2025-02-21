package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type ServerConfig struct {
	KubeConfigPath string `yaml:"kubeConfigPath"`
}

func ParseConfig(configPath string) (*ServerConfig, error) {
	sConfig := &ServerConfig{}

	// parse config file at configPath
	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(yamlFile, sConfig); err != nil {
		return nil, err
	}

	return sConfig, nil
}
