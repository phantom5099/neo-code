package todo

import "testing"

func TestParseTodoStatus(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   TodoStatus
		wantOK bool
	}{
		{name: "pending", in: "pending", want: TodoPending, wantOK: true},
		{name: "in_progress", in: "in_progress", want: TodoInProgress, wantOK: true},
		{name: "completed", in: "completed", want: TodoCompleted, wantOK: true},
		{name: "trim and lower", in: "  COMPLETED  ", want: TodoCompleted, wantOK: true},
		{name: "invalid", in: "done", want: "", wantOK: false},
		{name: "empty", in: "   ", want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseTodoStatus(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseTodoPriority(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   TodoPriority
		wantOK bool
	}{
		{name: "high", in: "high", want: TodoPriorityHigh, wantOK: true},
		{name: "medium", in: "medium", want: TodoPriorityMedium, wantOK: true},
		{name: "low", in: "low", want: TodoPriorityLow, wantOK: true},
		{name: "trim and lower", in: "\tHiGh\n", want: TodoPriorityHigh, wantOK: true},
		{name: "invalid", in: "urgent", want: "", wantOK: false},
		{name: "empty", in: "", want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseTodoPriority(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseTodoAction(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   TodoAction
		wantOK bool
	}{
		{name: "add", in: "add", want: TodoActionAdd, wantOK: true},
		{name: "update", in: "update", want: TodoActionUpdate, wantOK: true},
		{name: "list", in: "list", want: TodoActionList, wantOK: true},
		{name: "remove", in: "remove", want: TodoActionRemove, wantOK: true},
		{name: "clear", in: "clear", want: TodoActionClear, wantOK: true},
		{name: "trim and lower", in: "  LiSt  ", want: TodoActionList, wantOK: true},
		{name: "invalid", in: "toggle", want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseTodoAction(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
