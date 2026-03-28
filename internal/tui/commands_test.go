package tui

import "testing"

func TestMatchingSlashCommands(t *testing.T) {
	t.Parallel()

	app := App{}
	tests := []struct {
		name        string
		input       string
		expectCount int
		expectUsage string
	}{
		{
			name:        "non slash input returns no suggestions",
			input:       "hello",
			expectCount: 0,
		},
		{
			name:        "bare slash returns all commands",
			input:       "/",
			expectCount: len(builtinSlashCommands),
			expectUsage: slashUsageModel,
		},
		{
			name:        "prefix narrows suggestions",
			input:       "/mo",
			expectCount: 1,
			expectUsage: slashUsageModel,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := app.matchingSlashCommands(tt.input)
			if len(got) != tt.expectCount {
				t.Fatalf("expected %d suggestions, got %d", tt.expectCount, len(got))
			}
			if tt.expectUsage != "" && (len(got) == 0 || got[0].Command.Usage != tt.expectUsage && !containsUsage(got, tt.expectUsage)) {
				t.Fatalf("expected suggestions to contain %q, got %+v", tt.expectUsage, got)
			}
		})
	}
}

func containsUsage(suggestions []commandSuggestion, usage string) bool {
	for _, suggestion := range suggestions {
		if suggestion.Command.Usage == usage {
			return true
		}
	}
	return false
}
