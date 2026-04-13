package memo

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"neo-code/internal/config"
	providertypes "neo-code/internal/provider/types"
)

type stubMemoExtractor struct {
	mu        sync.Mutex
	callCount int
	calls     [][]providertypes.Message
	extractFn func(ctx context.Context, messages []providertypes.Message) ([]Entry, error)
}

func (s *stubMemoExtractor) Extract(ctx context.Context, messages []providertypes.Message) ([]Entry, error) {
	s.mu.Lock()
	s.callCount++
	s.calls = append(s.calls, cloneProviderMessages(messages))
	extractFn := s.extractFn
	s.mu.Unlock()

	if extractFn != nil {
		return extractFn(ctx, messages)
	}
	return nil, nil
}

func (s *stubMemoExtractor) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}

func newAutoExtractorTestService(t *testing.T) *Service {
	t.Helper()
	store := NewFileStore(t.TempDir(), t.TempDir())
	return NewService(store, nil, config.MemoConfig{MaxIndexLines: 200}, nil)
}

func TestAutoExtractorDebounceMergesRequests(t *testing.T) {
	svc := newAutoExtractorTestService(t)
	extractor := &stubMemoExtractor{
		extractFn: func(ctx context.Context, messages []providertypes.Message) ([]Entry, error) {
			last := messages[len(messages)-1].Content
			return []Entry{{Type: TypeProject, Title: last, Content: last, Source: SourceAutoExtract}}, nil
		},
	}
	auto := NewAutoExtractor(extractor, svc)
	auto.debounce = 20 * time.Millisecond
	auto.logf = func(string, ...any) {}

	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "first"}}, false)
	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "second"}}, false)

	waitFor(t, time.Second, func() bool { return extractor.Calls() == 1 })
	time.Sleep(60 * time.Millisecond)

	if extractor.Calls() != 1 {
		t.Fatalf("extractor calls = %d, want 1", extractor.Calls())
	}

	recall, err := svc.Recall(context.Background(), "second")
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if len(recall) != 1 {
		t.Fatalf("recall = %#v", recall)
	}
	for _, content := range recall {
		if !strings.Contains(content, "second") {
			t.Fatalf("recall content = %q", content)
		}
	}
}

func TestAutoExtractorSkipCancelsPendingRequest(t *testing.T) {
	svc := newAutoExtractorTestService(t)
	extractor := &stubMemoExtractor{}
	auto := NewAutoExtractor(extractor, svc)
	auto.debounce = 40 * time.Millisecond
	auto.logf = func(string, ...any) {}

	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "remember me"}}, false)
	time.Sleep(20 * time.Millisecond)
	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "remember me"}}, true)
	time.Sleep(80 * time.Millisecond)

	if extractor.Calls() != 0 {
		t.Fatalf("extractor calls = %d, want 0", extractor.Calls())
	}
}

func TestAutoExtractorTrailingRun(t *testing.T) {
	svc := newAutoExtractorTestService(t)
	firstStarted := make(chan struct{}, 1)
	secondStarted := make(chan struct{}, 1)
	releaseFirst := make(chan struct{})

	extractor := &stubMemoExtractor{
		extractFn: func(ctx context.Context, messages []providertypes.Message) ([]Entry, error) {
			switch messages[len(messages)-1].Content {
			case "first":
				firstStarted <- struct{}{}
				<-releaseFirst
			case "second":
				secondStarted <- struct{}{}
			}
			last := messages[len(messages)-1].Content
			return []Entry{{Type: TypeProject, Title: last, Content: last, Source: SourceAutoExtract}}, nil
		},
	}
	auto := NewAutoExtractor(extractor, svc)
	auto.debounce = 15 * time.Millisecond
	auto.logf = func(string, ...any) {}

	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "first"}}, false)

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first extraction did not start")
	}

	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "second"}}, false)
	time.Sleep(40 * time.Millisecond)
	close(releaseFirst)

	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("second trailing extraction did not start")
	}

	waitFor(t, time.Second, func() bool { return extractor.Calls() == 2 })
	waitFor(t, time.Second, func() bool {
		entries, err := svc.List(context.Background())
		return err == nil && len(entries) == 2
	})
}

func TestAutoExtractorSkipDoesNotQueue(t *testing.T) {
	svc := newAutoExtractorTestService(t)
	extractor := &stubMemoExtractor{}
	auto := NewAutoExtractor(extractor, svc)
	auto.debounce = 10 * time.Millisecond
	auto.logf = func(string, ...any) {}

	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "skip"}}, true)
	time.Sleep(50 * time.Millisecond)

	if extractor.Calls() != 0 {
		t.Fatalf("extractor calls = %d, want 0", extractor.Calls())
	}
}

func TestAutoExtractorErrorsAreSilent(t *testing.T) {
	svc := newAutoExtractorTestService(t)
	extractor := &stubMemoExtractor{
		extractFn: func(ctx context.Context, messages []providertypes.Message) ([]Entry, error) {
			return nil, errors.New("boom")
		},
	}
	auto := NewAutoExtractor(extractor, svc)
	auto.debounce = 10 * time.Millisecond
	auto.logf = func(string, ...any) {}

	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "x"}}, false)
	waitFor(t, time.Second, func() bool { return extractor.Calls() == 1 })

	entries, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %#v, want empty", entries)
	}
}

func TestAutoExtractorSuppressesExactDuplicates(t *testing.T) {
	svc := newAutoExtractorTestService(t)
	if err := svc.Add(context.Background(), Entry{
		Type:    TypeUser,
		Title:   "reply in chinese",
		Content: "reply in chinese",
		Source:  SourceAutoExtract,
	}); err != nil {
		t.Fatalf("seed Add() error = %v", err)
	}

	extractor := &stubMemoExtractor{
		extractFn: func(ctx context.Context, messages []providertypes.Message) ([]Entry, error) {
			return []Entry{
				{Type: TypeUser, Title: "reply in chinese", Content: "reply in chinese", Source: SourceAutoExtract},
				{Type: TypeFeedback, Title: "run tests first", Content: "run tests first", Source: SourceAutoExtract},
				{Type: TypeFeedback, Title: "run tests first", Content: "run tests first", Source: SourceAutoExtract},
			}, nil
		},
	}
	auto := NewAutoExtractor(extractor, svc)
	auto.debounce = 10 * time.Millisecond
	auto.logf = func(string, ...any) {}

	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "dedupe"}}, false)
	waitFor(t, time.Second, func() bool {
		entries, err := svc.List(context.Background())
		return err == nil && len(entries) == 2
	})

	entries, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
}

func TestAutoExtractorSuppressesExactDuplicatesAcrossSessions(t *testing.T) {
	svc := newAutoExtractorTestService(t)
	started := make(chan struct{}, 2)
	release := make(chan struct{})

	extractor := &stubMemoExtractor{
		extractFn: func(ctx context.Context, messages []providertypes.Message) ([]Entry, error) {
			started <- struct{}{}
			<-release
			return []Entry{
				{Type: TypeProject, Title: "same title", Content: "same content", Source: SourceAutoExtract},
			}, nil
		},
	}
	auto := NewAutoExtractor(extractor, svc)
	auto.debounce = 0
	auto.logf = func(string, ...any) {}

	auto.Schedule("session-1", []providertypes.Message{{Role: providertypes.RoleUser, Content: "one"}}, false)
	auto.Schedule("session-2", []providertypes.Message{{Role: providertypes.RoleUser, Content: "two"}}, false)

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("concurrent extraction did not start")
		}
	}
	close(release)

	waitFor(t, time.Second, func() bool { return extractor.Calls() == 2 })
	waitFor(t, time.Second, func() bool {
		entries, err := svc.List(context.Background())
		return err == nil && len(entries) == 1
	})

	entries, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
