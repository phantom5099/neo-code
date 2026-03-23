package configs

import (
	"fmt"
	"os"
	"strings"
)

const (
	DefaultPersonaFilePath = "./configs/persona.txt"
	legacyPersonaFilePath  = "./persona.txt"
)

func ResolvePersonaFilePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	candidates := []string{trimmed}
	if trimmed == legacyPersonaFilePath || trimmed == "persona.txt" {
		candidates = append(candidates, DefaultPersonaFilePath, "configs/persona.txt")
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return trimmed
}

func LoadPersonaPrompt(path string) (string, string, error) {
	resolvedPath := ResolvePersonaFilePath(path)
	if resolvedPath == "" {
		return "", "", nil
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", resolvedPath, fmt.Errorf("read persona file %q: %w", resolvedPath, err)
	}

	return strings.TrimSpace(string(data)), resolvedPath, nil
}
