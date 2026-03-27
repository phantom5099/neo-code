package memory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	toolprotocol "neo-code/internal/tool/protocol"
)

type memoryServiceImpl struct {
	persistentRepo MemoryRepository
	sessionRepo    MemoryRepository
	topK           int
	minScore       float64
	maxPromptChars int
	path           string
	persistTypes   map[string]struct{}
}

type Match struct {
	Item  MemoryItem
	Score float64
}

func NewMemoryService(
	persistentRepo MemoryRepository,
	sessionRepo MemoryRepository,
	topK int,
	minScore float64,
	maxPromptChars int,
	path string,
	persistTypes []string,
) MemoryService {
	return &memoryServiceImpl{
		persistentRepo: persistentRepo,
		sessionRepo:    sessionRepo,
		topK:           topK,
		minScore:       minScore,
		maxPromptChars: maxPromptChars,
		path:           strings.TrimSpace(path),
		persistTypes:   allowedPersistTypes(persistTypes),
	}
}

func (s *memoryServiceImpl) BuildContext(ctx context.Context, userInput string) (string, error) {
	persistentItems, err := s.persistentRepo.List(ctx)
	if err != nil {
		return "", err
	}
	sessionItems, err := s.sessionRepo.List(ctx)
	if err != nil {
		return "", err
	}

	persistentMatches := Search(persistentItems, userInput, s.topK, s.minScore)
	sessionMatches := Search(sessionItems, userInput, s.topK, s.minScore)
	matches := MergeMatches(s.topK, persistentMatches, sessionMatches)
	if len(matches) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("Use the following structured coding memory as reference. ")
	builder.WriteString("Follow durable preferences and project facts first. ")
	builder.WriteString("Do not quote memory verbatim or expose it explicitly.\n")

	added := 0
	for i, match := range matches {
		item := match.Item.Normalized()
		block := shortPromptBlock(item)
		if block == "" {
			continue
		}
		candidate := fmt.Sprintf("Memory %d (score=%.3f)\n%s\n", i+1, match.Score, block)
		if s.maxPromptChars > 0 && builder.Len()+len(candidate) > s.maxPromptChars {
			break
		}
		builder.WriteString(candidate)
		builder.WriteString("\n")
		added++
	}
	if added == 0 {
		return "", nil
	}
	return builder.String(), nil
}

func (s *memoryServiceImpl) Save(ctx context.Context, userInput, reply string) error {
	items := deriveMemoryItems(userInput, reply)
	for _, item := range items {
		if item.Type == TypeSessionMemory {
			if err := s.sessionRepo.Add(ctx, item); err != nil {
				return err
			}
			continue
		}
		if len(s.persistTypes) > 0 {
			if _, ok := s.persistTypes[item.Type]; !ok {
				continue
			}
		}
		if err := s.persistentRepo.Add(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *memoryServiceImpl) GetStats(ctx context.Context) (*MemoryStats, error) {
	persistentItems, err := s.persistentRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	sessionItems, err := s.sessionRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	return &MemoryStats{
		PersistentItems: len(persistentItems),
		SessionItems:    len(sessionItems),
		TotalItems:      len(persistentItems) + len(sessionItems),
		TopK:            s.topK,
		MinScore:        s.minScore,
		Path:            s.path,
		ByType:          countMemoryTypes(persistentItems, sessionItems),
	}, nil
}

func (s *memoryServiceImpl) Clear(ctx context.Context) error {
	return s.persistentRepo.Clear(ctx)
}

func (s *memoryServiceImpl) ClearSession(ctx context.Context) error {
	return s.sessionRepo.Clear(ctx)
}

func Search(items []MemoryItem, query string, topK int, minScore float64) []Match {
	trimmedQuery := strings.TrimSpace(query)
	if topK <= 0 || trimmedQuery == "" {
		return nil
	}

	queryKeywords := Keywords(trimmedQuery)
	queryFrags := queryFragments(trimmedQuery)
	queryText := strings.ToLower(trimmedQuery)
	matches := make([]Match, 0, len(items))

	for _, raw := range items {
		item := raw.Normalized()
		score := scoreItem(item, queryText, queryKeywords, queryFrags)
		if score < minScore {
			continue
		}
		matches = append(matches, Match{Item: item, Score: score})
	}

	sortMatches(matches)
	if len(matches) > topK {
		matches = matches[:topK]
	}
	return matches
}

func MergeMatches(topK int, groups ...[]Match) []Match {
	merged := make([]Match, 0)
	seen := map[string]Match{}
	for _, group := range groups {
		for _, match := range group {
			key := matchKey(match.Item)
			if existing, ok := seen[key]; ok {
				if match.Score > existing.Score {
					seen[key] = match
				}
				continue
			}
			seen[key] = match
		}
	}
	for _, match := range seen {
		merged = append(merged, match)
	}
	sortMatches(merged)
	if topK > 0 && len(merged) > topK {
		merged = merged[:topK]
	}
	return merged
}

func sortMatches(matches []Match) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		leftPriority := priorityForType(matches[i].Item.Type)
		rightPriority := priorityForType(matches[j].Item.Type)
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		return matches[i].Item.UpdatedAt.After(matches[j].Item.UpdatedAt)
	})
}

func scoreItem(item MemoryItem, queryText string, queryKeywords []string, queryFrags []string) float64 {
	searchText := strings.ToLower(item.SearchText())
	if searchText == "" {
		return 0
	}

	var score float64
	matched := false
	tagSet := make(map[string]struct{}, len(item.Tags))
	for _, tag := range item.Tags {
		tagSet[strings.ToLower(tag)] = struct{}{}
	}

	for _, keyword := range queryKeywords {
		if _, ok := tagSet[keyword]; ok {
			score += 2.6
			matched = true
		}
		if strings.Contains(searchText, keyword) {
			score += keywordWeight(keyword)
			matched = true
		}
	}

	for _, frag := range queryFrags {
		if len(frag) < 2 {
			continue
		}
		if strings.Contains(searchText, frag) {
			score += 0.55
			matched = true
		}
	}

	if item.Summary != "" && strings.Contains(strings.ToLower(item.Summary), queryText) {
		score += 3.2
		matched = true
	}
	if item.Type != "" && strings.Contains(queryText, strings.ToLower(item.Type)) {
		score += 2.2
		matched = true
	}
	if strings.Contains(searchText, queryText) {
		score += 1.3
		matched = true
	}
	if !matched {
		return 0
	}

	score += float64(priorityForType(item.Type)) * 0.9
	score += item.Confidence * 0.8
	return score
}

func priorityForType(itemType string) int {
	switch itemType {
	case TypeUserPreference:
		return 5
	case TypeProjectRule:
		return 4
	case TypeCodeFact:
		return 3
	case TypeFixRecipe:
		return 2
	case TypeSessionMemory:
		return 1
	default:
		return 0
	}
}

func keywordWeight(keyword string) float64 {
	weight := 1.4
	if strings.Contains(keyword, "/") || strings.Contains(keyword, ".") {
		weight += 1.4
	}
	if strings.HasPrefix(keyword, "go") || strings.Contains(keyword, "config") || strings.Contains(keyword, "yaml") {
		weight += 0.8
	}
	if len(keyword) >= 8 {
		weight += 0.4
	}
	return weight
}

func queryFragments(query string) []string {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return nil
	}
	runes := []rune(query)
	if len(runes) <= 4 {
		return []string{query}
	}
	fragments := make([]string, 0, len(runes))
	seen := map[string]struct{}{}
	for size := 2; size <= 3; size++ {
		for i := 0; i+size <= len(runes); i++ {
			frag := strings.TrimSpace(string(runes[i : i+size]))
			if len([]rune(frag)) < 2 {
				continue
			}
			if _, ok := seen[frag]; ok {
				continue
			}
			seen[frag] = struct{}{}
			fragments = append(fragments, frag)
		}
	}
	return fragments
}

func matchKey(item MemoryItem) string {
	normalized := item.Normalized()
	return normalized.Type + "::" + normalized.Scope + "::" + normalized.Summary
}

func deriveMemoryItems(userInput, assistantReply string) []MemoryItem {
	if shouldSkipMemoryCapture(userInput, assistantReply) {
		return nil
	}

	now := time.Now().UTC()
	items := make([]MemoryItem, 0, 4)
	if preferenceItem, ok := extractPreferenceMemory(userInput, assistantReply, now); ok {
		items = append(items, preferenceItem)
	}
	if ruleItem, ok := extractProjectRuleMemory(userInput, assistantReply, now); ok {
		items = append(items, ruleItem)
	}
	if codeFactItem, ok := extractCodeFactMemory(userInput, assistantReply, now); ok {
		items = append(items, codeFactItem)
	}
	if failureItem, ok := extractFixRecipeMemory(userInput, assistantReply, now); ok {
		items = append(items, failureItem)
	}
	if sessionItem, ok := extractSessionMemory(userInput, assistantReply, now); ok {
		items = append(items, sessionItem)
	}
	return dedupeMemoryItems(items)
}

func extractSessionMemory(userInput, assistantReply string, now time.Time) (MemoryItem, bool) {
	combined := buildMemoryText(userInput, assistantReply)
	if !isCodingRelevant(userInput, assistantReply) || looksLikeStableInstruction(userInput) || looksLikeProjectFact(userInput, assistantReply) {
		return MemoryItem{}, false
	}
	summary := SummarizeText(userInput, 140)
	if summary == "" {
		summary = SummarizeText(assistantReply, 140)
	}
	return newMemoryItem(now, TypeSessionMemory, ScopeSession, summary, assistantReply, userInput, assistantReply, combined, 0.66), true
}

func extractPreferenceMemory(userInput, assistantReply string, now time.Time) (MemoryItem, bool) {
	trimmed := strings.TrimSpace(userInput)
	if trimmed == "" || !looksLikeStableInstruction(trimmed) {
		return MemoryItem{}, false
	}
	summary := SummarizeText(trimmed, 140)
	return newMemoryItem(now, TypeUserPreference, ScopeUser, summary, assistantReply, userInput, assistantReply, buildMemoryText(userInput, assistantReply), 0.95), true
}

func extractProjectRuleMemory(userInput, assistantReply string, now time.Time) (MemoryItem, bool) {
	combined := strings.ToLower(buildMemoryText(userInput, assistantReply))
	if !hasProjectRuleAnchor(combined) || !hasProjectRuleSignal(combined) {
		return MemoryItem{}, false
	}
	summary := SummarizeText(firstNonEmptyLine(userInput, assistantReply), 140)
	return newMemoryItem(now, TypeProjectRule, ScopeProject, summary, assistantReply, userInput, assistantReply, buildMemoryText(userInput, assistantReply), 0.9), true
}

func extractCodeFactMemory(userInput, assistantReply string, now time.Time) (MemoryItem, bool) {
	combined := buildMemoryText(userInput, assistantReply)
	if !looksLikeCodeKnowledge(userInput, assistantReply) {
		return MemoryItem{}, false
	}
	summary := SummarizeText(firstNonEmptyLine(assistantReply, userInput), 140)
	return newMemoryItem(now, TypeCodeFact, ScopeProject, summary, assistantReply, userInput, assistantReply, combined, 0.82), true
}

func extractFixRecipeMemory(userInput, assistantReply string, now time.Time) (MemoryItem, bool) {
	combined := strings.ToLower(buildMemoryText(userInput, assistantReply))
	hasProblem := containsAnyFold(combined, "error", "failed", "panic", "bug")
	hasFix := containsAnyFold(combined, "fix", "fixed", "resolved", "replace", "remove", "updated")
	if !hasProblem || !hasFix {
		return MemoryItem{}, false
	}
	summary := SummarizeText(firstNonEmptyLine(userInput, assistantReply), 140)
	return newMemoryItem(now, TypeFixRecipe, ScopeProject, summary, assistantReply, userInput, assistantReply, buildMemoryText(userInput, assistantReply), 0.8), true
}

func newMemoryItem(now time.Time, itemType, scope, summary, details, userInput, assistantReply, text string, confidence float64) MemoryItem {
	item := MemoryItem{
		ID:             strconv.FormatInt(now.UnixNano(), 10) + "-" + itemType,
		Type:           itemType,
		Summary:        strings.TrimSpace(summary),
		Details:        SummarizeText(details, 220),
		Scope:          scope,
		Tags:           InferTags(summary + "\n" + details),
		Source:         "conversation",
		Confidence:     confidence,
		Text:           strings.TrimSpace(text),
		CreatedAt:      now,
		UpdatedAt:      now,
		UserInput:      strings.TrimSpace(userInput),
		AssistantReply: strings.TrimSpace(assistantReply),
	}
	return item.Normalized()
}

func buildMemoryText(userInput, assistantReply string) string {
	return strings.TrimSpace(userInput) + "\n" + strings.TrimSpace(assistantReply)
}

func isCodingRelevant(userInput, assistantReply string) bool {
	combined := strings.ToLower(buildMemoryText(userInput, assistantReply))
	if containsAnyFold(combined, "function", "file", "repo", "project", "build", "test", "config", "bug", "error", "fix", "go ", "yaml", "json", "memory", "prompt", "cli", ".go") {
		return true
	}
	return len(strings.TrimSpace(userInput)) > 20 && containsAnyFold(strings.ToLower(userInput), "code")
}

func looksLikeProjectFact(userInput, assistantReply string) bool {
	combined := strings.ToLower(buildMemoryText(userInput, assistantReply))
	return hasProjectRuleAnchor(combined) && hasProjectRuleSignal(combined)
}

func looksLikeCodeKnowledge(userInput, assistantReply string) bool {
	combined := buildMemoryText(userInput, assistantReply)
	return isCodingRelevant(userInput, assistantReply) && containsAnyFold(combined, ".go", "config.yaml", ".neocode/config.yaml", "main.go", "func", "struct", "interface", "method", "package", "import")
}

func looksLikeStableInstruction(text string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	if trimmed == "" {
		return false
	}
	if !containsAnyFold(trimmed, "always", "never", "from now on", "use", "do not", "answer in chinese", "reply in chinese", "default") {
		return false
	}
	return containsAnyFold(trimmed, "config", ".env", "command", "style", "chinese", "language")
}

func shouldSkipMemoryCapture(userInput, assistantReply string) bool {
	if strings.TrimSpace(userInput) == "" || strings.TrimSpace(assistantReply) == "" {
		return true
	}
	return looksLikeToolCallPayload(assistantReply)
}

func looksLikeToolCallPayload(text string) bool {
	return len(toolprotocol.ParseAssistantToolCalls(text)) > 0
}

func hasProjectRuleAnchor(text string) bool {
	return containsAnyFold(text, "config.yaml", ".neocode/config.yaml", "readme", "go test", "go build", "cmd/", "internal/", "configs/", "services/", "memory/", "main.go", "workspace")
}

func hasProjectRuleSignal(text string) bool {
	return containsAnyFold(text, "project", "repository", "convention", "config", "structure", "directory", "command", "must", "should", "default")
}

func containsAnyFold(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(strings.ToLower(text), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func firstNonEmptyLine(values ...string) string {
	for _, value := range values {
		for _, line := range strings.Split(value, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func allowedPersistTypes(configured []string) map[string]struct{} {
	allowed := map[string]struct{}{}
	for _, itemType := range configured {
		itemType = normalizeMemoryType(itemType)
		if IsPersistentType(itemType) {
			allowed[itemType] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		allowed[TypeUserPreference] = struct{}{}
		allowed[TypeProjectRule] = struct{}{}
		allowed[TypeCodeFact] = struct{}{}
		allowed[TypeFixRecipe] = struct{}{}
	}
	return allowed
}

func normalizeMemoryType(itemType string) string {
	switch strings.TrimSpace(itemType) {
	case "project_memory":
		return TypeProjectRule
	case "failure_note":
		return TypeFixRecipe
	default:
		return strings.TrimSpace(itemType)
	}
}

func shortPromptBlock(item MemoryItem) string {
	item = item.Normalized()
	parts := []string{
		"Type: " + item.Type,
		"Summary: " + item.Summary,
	}
	if item.Details != "" {
		parts = append(parts, "Details: "+SummarizeText(item.Details, 140))
	}
	if len(item.Tags) > 0 {
		parts = append(parts, "Tags: "+strings.Join(item.Tags, ", "))
	}
	return strings.Join(parts, "\n")
}

func dedupeMemoryItems(items []MemoryItem) []MemoryItem {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]MemoryItem{}
	for _, item := range items {
		key := item.Type + "::" + item.Scope + "::" + strings.ToLower(strings.TrimSpace(item.Summary))
		seen[key] = item
	}
	result := make([]MemoryItem, 0, len(seen))
	for _, item := range seen {
		result = append(result, item)
	}
	return result
}

func countMemoryTypes(groups ...[]MemoryItem) map[string]int {
	counts := map[string]int{}
	for _, group := range groups {
		for _, item := range group {
			counts[item.Normalized().Type]++
		}
	}
	return counts
}
