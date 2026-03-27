package core

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEnterSubmitsMessage(t *testing.T) {
	client := &fakeChatClient{chatChunks: []string{"ok"}}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true
	m.textarea.SetValue("hello")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if cmd == nil {
		t.Fatal("expected Enter to submit and start streaming")
	}
	msg := cmd()
	updated, cmd = got.Update(msg)
	got = updated.(Model)
	if len(got.chat.Messages) != 2 || got.chat.Messages[0].Role != "user" {
		t.Fatalf("expected submitted messages, got %+v", got.chat.Messages)
	}
}

func TestAltEnterInsertsNewline(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.textarea.SetValue("line one")
	m.textarea.CursorEnd()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	got := updated.(Model)

	if cmd == nil {
		t.Fatal("expected textarea update command for Alt+Enter")
	}
	if got.textarea.Value() != "line one\n" {
		t.Fatalf("expected Alt+Enter to insert newline, got %q", got.textarea.Value())
	}
	if len(got.chat.Messages) != 0 {
		t.Fatalf("expected no submitted messages, got %+v", got.chat.Messages)
	}
}
