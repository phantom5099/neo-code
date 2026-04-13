package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"neo-code/internal/config"
	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

type stubTextGenProvider struct {
	requests []providertypes.GenerateRequest
	generate func(ctx context.Context, req providertypes.GenerateRequest, events chan<- providertypes.StreamEvent) error
}

func (s *stubTextGenProvider) Generate(
	ctx context.Context,
	req providertypes.GenerateRequest,
	events chan<- providertypes.StreamEvent,
) error {
	s.requests = append(s.requests, req)
	if s.generate != nil {
		return s.generate(ctx, req, events)
	}
	return nil
}

type stubTextGenFactory struct {
	provider provider.Provider
	err      error
	calls    int
	configs  []provider.RuntimeConfig
}

func (s *stubTextGenFactory) Build(ctx context.Context, cfg provider.RuntimeConfig) (provider.Provider, error) {
	s.calls++
	s.configs = append(s.configs, cfg)
	if s.err != nil {
		return nil, s.err
	}
	return s.provider, nil
}

// TestProviderTextGeneratorGenerateSuccess 验证文本生成器可以聚合流式文本并且不传 tools。
func TestProviderTextGeneratorGenerateSuccess(t *testing.T) {
	manager := newLoadedConfigManagerForTextGenerator(t)
	providerStub := &stubTextGenProvider{
		generate: func(ctx context.Context, req providertypes.GenerateRequest, events chan<- providertypes.StreamEvent) error {
			events <- providertypes.NewTextDeltaStreamEvent("hello ")
			events <- providertypes.NewTextDeltaStreamEvent("world")
			events <- providertypes.NewMessageDoneStreamEvent("stop", nil)
			return nil
		},
	}
	factory := &stubTextGenFactory{provider: providerStub}
	generator := newProviderTextGenerator(factory, manager)

	text, err := generator.Generate(context.Background(), "memo system prompt", []providertypes.Message{
		{Role: providertypes.RoleUser, Content: "记住这个。"},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if text != "hello world" {
		t.Fatalf("text = %q, want %q", text, "hello world")
	}
	if factory.calls != 1 {
		t.Fatalf("Build() calls = %d, want 1", factory.calls)
	}
	if len(providerStub.requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(providerStub.requests))
	}
	request := providerStub.requests[0]
	if request.SystemPrompt != "memo system prompt" {
		t.Fatalf("SystemPrompt = %q", request.SystemPrompt)
	}
	if len(request.Tools) != 0 {
		t.Fatalf("Tools = %#v, want empty", request.Tools)
	}
	if request.Messages[0].Content != "记住这个。" {
		t.Fatalf("Messages = %#v", request.Messages)
	}
}

// TestProviderTextGeneratorGenerateBuildFailure 验证 provider 构建失败会原样返回。
func TestProviderTextGeneratorGenerateBuildFailure(t *testing.T) {
	manager := newLoadedConfigManagerForTextGenerator(t)
	factory := &stubTextGenFactory{err: errors.New("build failed")}
	generator := newProviderTextGenerator(factory, manager)

	_, err := generator.Generate(context.Background(), "prompt", nil)
	if err == nil || !strings.Contains(err.Error(), "build failed") {
		t.Fatalf("Generate() error = %v", err)
	}
}

// TestProviderTextGeneratorGenerateFailure 验证 provider.Generate 失败会向上返回。
func TestProviderTextGeneratorGenerateFailure(t *testing.T) {
	manager := newLoadedConfigManagerForTextGenerator(t)
	providerStub := &stubTextGenProvider{
		generate: func(ctx context.Context, req providertypes.GenerateRequest, events chan<- providertypes.StreamEvent) error {
			return errors.New("generate failed")
		},
	}
	generator := newProviderTextGenerator(&stubTextGenFactory{provider: providerStub}, manager)

	_, err := generator.Generate(context.Background(), "prompt", nil)
	if err == nil || !strings.Contains(err.Error(), "generate failed") {
		t.Fatalf("Generate() error = %v", err)
	}
}

// TestProviderTextGeneratorGenerateRequiresMessageDone 验证缺失 message_done 会视为错误。
func TestProviderTextGeneratorGenerateRequiresMessageDone(t *testing.T) {
	manager := newLoadedConfigManagerForTextGenerator(t)
	providerStub := &stubTextGenProvider{
		generate: func(ctx context.Context, req providertypes.GenerateRequest, events chan<- providertypes.StreamEvent) error {
			events <- providertypes.NewTextDeltaStreamEvent("partial")
			return nil
		},
	}
	generator := newProviderTextGenerator(&stubTextGenFactory{provider: providerStub}, manager)

	_, err := generator.Generate(context.Background(), "prompt", nil)
	if err == nil || !strings.Contains(err.Error(), "message_done") {
		t.Fatalf("Generate() error = %v", err)
	}
}

// newLoadedConfigManagerForTextGenerator 创建带默认 provider 选择的配置管理器。
func newLoadedConfigManagerForTextGenerator(t *testing.T) *config.Manager {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.OpenAIDefaultAPIKeyEnv, "test-key")

	defaults := config.StaticDefaults()
	defaults.Workdir = t.TempDir()
	loader := config.NewLoader("", defaults)
	manager := config.NewManager(loader)
	if _, err := manager.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Update(context.Background(), func(cfg *config.Config) error {
		cfg.SelectedProvider = config.OpenAIName
		cfg.CurrentModel = config.OpenAIDefaultModel
		return nil
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	return manager
}
