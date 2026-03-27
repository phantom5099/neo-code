package security

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ConfigLoader struct{}

func (l ConfigLoader) LoadDir(configDir string) (*Config, *Config, *Config, error) {
	blacklist, err := l.loadFile(filepath.Join(configDir, "blacklist.yaml"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load blacklist: %w", err)
	}

	whitelist, err := l.loadFile(filepath.Join(configDir, "whitelist.yaml"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load whitelist: %w", err)
	}

	yellowlist, err := l.loadFile(filepath.Join(configDir, "yellowlist.yaml"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load yellowlist: %w", err)
	}

	return blacklist, whitelist, yellowlist, nil
}

func (l ConfigLoader) loadFile(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Rules: []Rule{}}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", filePath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", filePath, err)
	}
	if config.Rules == nil {
		config.Rules = []Rule{}
	}
	return &config, nil
}
