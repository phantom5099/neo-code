package session

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"neo-code/internal/agentruntime/memory"
)

var fileRefPattern = regexp.MustCompile(`(?i)(?:[a-z]:\\|\./|\.\\|/)?[a-z0-9_./\\-]+\.[a-z0-9]+`)

type workingMemoryServiceImpl struct {
	repo             WorkingMemoryRepository
	maxRecentTurns   int
	maxOpenQuestions int
	maxRecentFiles   int
	workspaceRoot    string
}

func NewWorkingMemoryService(repo WorkingMemoryRepository, maxRecentTurns int, workspaceRoot string) WorkingMemoryService {
	if maxRecentTurns <= 0 {
		maxRecentTurns = 6
	}
	return &workingMemoryServiceImpl{
		repo:             repo,
		maxRecentTurns:   maxRecentTurns,
		maxOpenQuestions: 3,
		maxRecentFiles:   6,
		workspaceRoot:    strings.TrimSpace(workspaceRoot),
	}
}

func (s *workingMemoryServiceImpl) BuildContext(ctx context.Context, messages []Message) (string, error) {
	if err := s.Refresh(ctx, messages); err != nil {
		return "", err
	}
	state, err := s.Get(ctx)
	if err != nil {
		return "", err
	}
	return formatWorkingMemoryContext(state, s.workspaceRoot), nil
}

func (s *workingMemoryServiceImpl) Refresh(ctx context.Context, messages []Message) error {
	return s.repo.Save(ctx, s.buildState(messages))
}

func (s *workingMemoryServiceImpl) Clear(ctx context.Context) error {
	return s.repo.Clear(ctx)
}

func (s *workingMemoryServiceImpl) Get(ctx context.Context) (*WorkingMemoryState, error) {
	return s.repo.Get(ctx)
}

func (s *workingMemoryServiceImpl) buildState(messages []Message) *WorkingMemoryState {
	turns := collectRecentTurns(messages)
	if len(turns) > s.maxRecentTurns {
		turns = turns[len(turns)-s.maxRecentTurns:]
	}

	currentTask := latestUserMessage(messages)
	openQuestions := collectOpenQuestions(messages, s.maxOpenQuestions)
	return &WorkingMemoryState{
		CurrentTask:         currentTask,
		TaskSummary:         buildTaskSummary(turns, currentTask),
		LastCompletedAction: inferLastCompletedAction(messages),
		CurrentInProgress:   inferCurrentInProgress(messages, currentTask),
		NextStep:            inferNextStep(messages, openQuestions, currentTask),
		RecentTurns:         turns,
		OpenQuestions:       openQuestions,
		RecentFiles:         collectRecentFiles(messages, s.maxRecentFiles),
		UpdatedAt:           time.Now().UTC(),
	}
}

func collectRecentTurns(messages []Message) []WorkingMemoryTurn {
	turns := make([]WorkingMemoryTurn, 0)
	var pendingUser string
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			pendingUser = strings.TrimSpace(msg.Content)
		case "assistant":
			assistant := strings.TrimSpace(msg.Content)
			if pendingUser == "" && assistant == "" {
				continue
			}
			turns = append(turns, WorkingMemoryTurn{
				User:      pendingUser,
				Assistant: assistant,
			})
			pendingUser = ""
		}
	}
	if pendingUser != "" {
		turns = append(turns, WorkingMemoryTurn{User: pendingUser})
	}
	return turns
}

func latestUserMessage(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func buildTaskSummary(turns []WorkingMemoryTurn, currentTask string) string {
	if strings.TrimSpace(currentTask) != "" {
		return memory.SummarizeText(currentTask, 160)
	}
	if len(turns) == 0 {
		return ""
	}
	latest := turns[len(turns)-1]
	if latest.User != "" {
		return memory.SummarizeText(latest.User, 160)
	}
	return memory.SummarizeText(latest.Assistant, 160)
}

func inferLastCompletedAction(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "assistant" {
			continue
		}
		for _, line := range strings.Split(msg.Content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if containsAnyFold(line, "completed", "implemented", "fixed", "updated", "added", "created") {
				return memory.SummarizeText(line, 140)
			}
		}
	}
	return ""
}

func inferCurrentInProgress(messages []Message, currentTask string) string {
	trimmedTask := strings.TrimSpace(currentTask)
	if trimmedTask == "" {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if containsAnyFold(content, "working on", "next", "continue", "in progress") {
			return memory.SummarizeText(content, 140)
		}
		break
	}
	return memory.SummarizeText(trimmedTask, 140)
}

func inferNextStep(messages []Message, openQuestions []string, currentTask string) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "assistant" {
			continue
		}
		for _, line := range strings.Split(msg.Content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if containsAnyFold(line, "next step", "next", "follow-up", "continue") {
				return memory.SummarizeText(line, 140)
			}
		}
	}
	if len(openQuestions) > 0 {
		return "Resolve first: " + memory.SummarizeText(openQuestions[0], 110)
	}
	if strings.TrimSpace(currentTask) != "" {
		return "Continue: " + memory.SummarizeText(currentTask, 110)
	}
	return ""
}

func collectOpenQuestions(messages []Message, limit int) []string {
	questions := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for i := len(messages) - 1; i >= 0 && len(questions) < limit; i-- {
		msg := messages[i]
		if msg.Role != "user" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" || !looksLikeOpenQuestion(content) {
			continue
		}
		key := strings.ToLower(content)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		questions = append(questions, memory.SummarizeText(content, 120))
	}
	return reverseStrings(questions)
}

func collectRecentFiles(messages []Message, limit int) []string {
	files := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for i := len(messages) - 1; i >= 0 && len(files) < limit; i-- {
		matches := fileRefPattern.FindAllString(messages[i].Content, -1)
		for _, match := range matches {
			normalized := strings.TrimSpace(strings.ReplaceAll(match, "\\", "/"))
			if normalized == "" {
				continue
			}
			lowered := strings.ToLower(normalized)
			if _, ok := seen[lowered]; ok {
				continue
			}
			seen[lowered] = struct{}{}
			files = append(files, normalized)
			if len(files) >= limit {
				break
			}
		}
	}
	return reverseStrings(files)
}

func looksLikeOpenQuestion(text string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	if trimmed == "" {
		return false
	}
	if strings.ContainsAny(trimmed, "?？") {
		return true
	}
	return containsAnyFold(trimmed, "how", "why", "where", "which", "what")
}

func formatWorkingMemoryContext(state *WorkingMemoryState, workspaceRoot string) string {
	if state == nil {
		return ""
	}

	parts := make([]string, 0, 8)
	if strings.TrimSpace(workspaceRoot) != "" {
		parts = append(parts, "Workspace root: "+workspaceRoot)
	}
	if state.CurrentTask != "" {
		parts = append(parts, "Current task: "+memory.SummarizeText(state.CurrentTask, 180))
	}
	if state.TaskSummary != "" {
		parts = append(parts, "Task summary: "+state.TaskSummary)
	}
	if state.LastCompletedAction != "" {
		parts = append(parts, "Last completed action: "+state.LastCompletedAction)
	}
	if state.CurrentInProgress != "" {
		parts = append(parts, "Current in progress: "+state.CurrentInProgress)
	}
	if state.NextStep != "" {
		parts = append(parts, "Next step: "+state.NextStep)
	}
	if len(state.OpenQuestions) > 0 {
		parts = append(parts, "Open questions: "+strings.Join(state.OpenQuestions, " | "))
	}
	if len(state.RecentFiles) > 0 {
		parts = append(parts, "Recent file refs: "+strings.Join(state.RecentFiles, ", "))
	}
	if !state.UpdatedAt.IsZero() {
		parts = append(parts, "State updated at: "+state.UpdatedAt.Format(time.RFC3339))
	}
	if len(state.RecentTurns) > 0 {
		lines := make([]string, 0, len(state.RecentTurns))
		for i, turn := range state.RecentTurns {
			lines = append(lines, fmt.Sprintf(
				"Turn %d user=%q assistant=%q",
				i+1,
				memory.SummarizeText(turn.User, 100),
				memory.SummarizeText(turn.Assistant, 100),
			))
		}
		parts = append(parts, "Recent turns:\n"+strings.Join(lines, "\n"))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Use the following working memory to stay consistent with the active task and recent context. Prefer it over reconstructing context from scratch.\n" + strings.Join(parts, "\n")
}

func reverseStrings(values []string) []string {
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}
	return values
}

func containsAnyFold(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(strings.ToLower(text), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
