package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type WorkingMemoryStore struct {
	mu    sync.RWMutex
	state *WorkingMemoryState
	path  string
}

func NewWorkingMemoryStore(path ...string) *WorkingMemoryStore {
	storePath := ""
	if len(path) > 0 {
		storePath = strings.TrimSpace(path[0])
	}
	return &WorkingMemoryStore{path: storePath}
}

func (s *WorkingMemoryStore) Get(ctx context.Context) (*WorkingMemoryState, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil && strings.TrimSpace(s.path) != "" {
		state, err := s.readLocked()
		if err != nil {
			return nil, err
		}
		s.state = state
	}
	if s.state == nil {
		return &WorkingMemoryState{}, nil
	}
	return cloneWorkingMemoryState(s.state), nil
}

func (s *WorkingMemoryStore) Save(ctx context.Context, state *WorkingMemoryState) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = cloneWorkingMemoryState(state)
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	return s.writeLocked(s.state)
}

func (s *WorkingMemoryStore) Clear(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = nil
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *WorkingMemoryStore) readLocked() (*WorkingMemoryState, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &WorkingMemoryState{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return &WorkingMemoryState{}, nil
	}

	var state WorkingMemoryState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return cloneWorkingMemoryState(&state), nil
}

func (s *WorkingMemoryStore) writeLocked(state *WorkingMemoryState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "working-memory-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err == nil {
		return nil
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func cloneWorkingMemoryState(state *WorkingMemoryState) *WorkingMemoryState {
	if state == nil {
		return &WorkingMemoryState{}
	}
	cloned := &WorkingMemoryState{
		CurrentTask:         state.CurrentTask,
		TaskSummary:         state.TaskSummary,
		LastCompletedAction: state.LastCompletedAction,
		CurrentInProgress:   state.CurrentInProgress,
		NextStep:            state.NextStep,
		UpdatedAt:           state.UpdatedAt,
	}
	if len(state.RecentTurns) > 0 {
		cloned.RecentTurns = make([]WorkingMemoryTurn, len(state.RecentTurns))
		copy(cloned.RecentTurns, state.RecentTurns)
	}
	if len(state.OpenQuestions) > 0 {
		cloned.OpenQuestions = make([]string, len(state.OpenQuestions))
		copy(cloned.OpenQuestions, state.OpenQuestions)
	}
	if len(state.RecentFiles) > 0 {
		cloned.RecentFiles = make([]string, len(state.RecentFiles))
		copy(cloned.RecentFiles, state.RecentFiles)
	}
	return cloned
}
