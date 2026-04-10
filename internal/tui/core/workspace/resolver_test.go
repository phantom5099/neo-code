package workspace

import (
	"testing"
)

func TestSelectSessionWorkdir(t *testing.T) {
	tests := []struct {
		name           string
		sessionWorkdir string
		defaultWorkdir string
		want           string
	}{
		{"prefer session workdir", "/session", "/default", "/session"},
		{"fallback to default", "", "/default", "/default"},
		{"both empty", "", "", ""},
		{"session with whitespace", "  /session  ", "/default", "/session"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SelectSessionWorkdir(tt.sessionWorkdir, tt.defaultWorkdir); got != tt.want {
				t.Errorf("SelectSessionWorkdir() = %v, want %v", got, tt.want)
			}
		})
	}
}
