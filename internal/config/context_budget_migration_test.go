package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateContextBudgetConfigContentMovesAutoCompactToBudget(t *testing.T) {
	t.Parallel()

	input := []byte(strings.TrimSpace(`
selected_provider: openai
context:
  compact:
    manual_strategy: keep_recent
  auto_compact:
    input_token_threshold: 120000
    reserve_tokens: 13000
    fallback_input_token_threshold: 100000
`) + "\n")

	out, changed, notes, err := MigrateContextBudgetConfigContent(input)
	if err != nil {
		t.Fatalf("MigrateContextBudgetConfigContent() error = %v", err)
	}
	if !changed {
		t.Fatal("expected migration change")
	}
	if len(notes) != 0 {
		t.Fatalf("expected no migration notes, got %v", notes)
	}
	text := string(out)
	if strings.Contains(text, "auto_compact:") {
		t.Fatalf("expected auto_compact removed, got:\n%s", text)
	}
	for _, want := range []string{
		"budget:",
		"prompt_budget: 120000",
		"reserve_tokens: 13000",
		"fallback_prompt_budget: 100000",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected migrated YAML to contain %q, got:\n%s", want, text)
		}
	}
}

func TestMigrateContextBudgetConfigContentRejectsMixedBudgetBlocks(t *testing.T) {
	t.Parallel()

	input := []byte(strings.TrimSpace(`
context:
  budget:
    prompt_budget: 100000
  auto_compact:
    input_token_threshold: 120000
`) + "\n")

	_, _, _, err := MigrateContextBudgetConfigContent(input)
	if err == nil || !strings.Contains(err.Error(), "cannot both exist") {
		t.Fatalf("expected mixed block error, got %v", err)
	}
}

func TestMigrateContextBudgetConfigContentAddsNoteWhenEnabledExplicitlyFalse(t *testing.T) {
	t.Parallel()

	input := []byte(strings.TrimSpace(`
context:
  auto_compact:
    enabled: false
    input_token_threshold: 120000
`) + "\n")

	_, changed, notes, err := MigrateContextBudgetConfigContent(input)
	if err != nil {
		t.Fatalf("MigrateContextBudgetConfigContent() error = %v", err)
	}
	if !changed {
		t.Fatal("expected migration change")
	}
	if len(notes) != 1 || notes[0] != ContextBudgetMigrationNoteEnabledDeprecated {
		t.Fatalf("expected notes [%q], got %v", ContextBudgetMigrationNoteEnabledDeprecated, notes)
	}
}

func TestMigrateContextBudgetConfigContentNoNoteWhenEnabledTrueOrMissing(t *testing.T) {
	t.Parallel()

	cases := []string{
		strings.TrimSpace(`
context:
  auto_compact:
    enabled: true
    reserve_tokens: 13000
`) + "\n",
		strings.TrimSpace(`
context:
  auto_compact:
    reserve_tokens: 13000
`) + "\n",
	}

	for _, input := range cases {
		_, changed, notes, err := MigrateContextBudgetConfigContent([]byte(input))
		if err != nil {
			t.Fatalf("MigrateContextBudgetConfigContent() error = %v", err)
		}
		if !changed {
			t.Fatal("expected migration change")
		}
		if len(notes) != 0 {
			t.Fatalf("expected no notes, got %v", notes)
		}
	}
}

func TestMigrateContextBudgetConfigFileCreatesBackup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, configName)
	original := strings.TrimSpace(`
context:
  auto_compact:
    input_token_threshold: 120000
`) + "\n"
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	result, err := MigrateContextBudgetConfigFile(target, false)
	if err != nil {
		t.Fatalf("MigrateContextBudgetConfigFile() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("expected changed result")
	}
	if len(result.Notes) != 0 {
		t.Fatalf("expected no notes, got %v", result.Notes)
	}
	if result.Backup == "" {
		t.Fatal("expected backup path")
	}
	backup, err := os.ReadFile(result.Backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != original {
		t.Fatalf("expected backup to keep original content, got:\n%s", backup)
	}
}

func TestUpgradeConfigSchemaReturnsNotes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, configName)
	original := strings.TrimSpace(`
context:
  auto_compact:
    enabled: false
    reserve_tokens: 13000
`) + "\n"
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	result, err := UpgradeConfigSchema(target)
	if err != nil {
		t.Fatalf("UpgradeConfigSchema() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("expected changed result")
	}
	if len(result.Notes) != 1 || result.Notes[0] != ContextBudgetMigrationNoteEnabledDeprecated {
		t.Fatalf("expected note %q, got %v", ContextBudgetMigrationNoteEnabledDeprecated, result.Notes)
	}
}
