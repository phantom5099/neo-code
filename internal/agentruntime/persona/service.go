package persona

import (
	"context"
	"strings"

	"neo-code/internal/config"
)

// Service provides the active persona prompt for runtime requests.
type Service interface {
	GetActivePrompt(ctx context.Context) (string, error)
}

type fileService struct {
	path string
}

// NewFileService loads the active persona prompt from the configured file path.
func NewFileService(path string) Service {
	return &fileService{path: strings.TrimSpace(path)}
}

func (s *fileService) GetActivePrompt(ctx context.Context) (string, error) {
	_ = ctx
	if s == nil || strings.TrimSpace(s.path) == "" {
		return "", nil
	}

	prompt, _, err := config.LoadPersonaPrompt(s.path)
	if err != nil {
		return "", nil
	}
	return prompt, nil
}
