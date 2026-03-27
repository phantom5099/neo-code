package config

import (
	"os"
	"path/filepath"
	"strings"
)

const AppHomeDirName = ".neocode"

func AppHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return AppHomeDirName
	}
	return filepath.Join(home, AppHomeDirName)
}

func DefaultConfigPath() string {
	return filepath.Join(AppHomeDir(), "config.yaml")
}

func DefaultDataDir() string {
	return filepath.Join(AppHomeDir(), "data")
}

func DefaultMemoryStoragePath() string {
	return filepath.Join(DefaultDataDir(), "memory_rules.json")
}

func DefaultWorkspaceStateDir() string {
	return filepath.Join(DefaultDataDir(), "workspaces")
}

func DefaultPersonaPath() string {
	return filepath.Join(AppHomeDir(), "persona.txt")
}
