package cli

import (
	"bytes"
	"context"
	"testing"

	"neo-code/internal/updater"
)

func TestUpdateCommandPassesPrereleaseFlag(t *testing.T) {
	originalRunner := runUpdateCommand
	originalPreload := runGlobalPreload
	originalSilentCheck := runSilentUpdateCheck
	t.Cleanup(func() { runUpdateCommand = originalRunner })
	t.Cleanup(func() { runGlobalPreload = originalPreload })
	t.Cleanup(func() { runSilentUpdateCheck = originalSilentCheck })

	runGlobalPreload = func(context.Context) error { return nil }
	runSilentUpdateCheck = func(context.Context) {}

	var received updateCommandOptions
	runUpdateCommand = func(_ context.Context, options updateCommandOptions) (updater.UpdateResult, error) {
		received = options
		return updater.UpdateResult{Updated: false, LatestVersion: "v0.2.1"}, nil
	}

	command := NewRootCommand()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"update", "--prerelease"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	if !received.IncludePrerelease {
		t.Fatal("expected IncludePrerelease to be true")
	}
	if got := stdout.String(); got == "" {
		t.Fatal("expected update command output")
	}
}

func TestUpdateCommandShowsSuccessMessage(t *testing.T) {
	originalRunner := runUpdateCommand
	originalPreload := runGlobalPreload
	originalSilentCheck := runSilentUpdateCheck
	t.Cleanup(func() { runUpdateCommand = originalRunner })
	t.Cleanup(func() { runGlobalPreload = originalPreload })
	t.Cleanup(func() { runSilentUpdateCheck = originalSilentCheck })

	runGlobalPreload = func(context.Context) error { return nil }
	runSilentUpdateCheck = func(context.Context) {}
	runUpdateCommand = func(context.Context, updateCommandOptions) (updater.UpdateResult, error) {
		return updater.UpdateResult{
			CurrentVersion: "v0.1.0",
			LatestVersion:  "v0.2.1",
			Updated:        true,
		}, nil
	}

	command := NewRootCommand()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"update"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	if got := stdout.String(); got == "" || !bytes.Contains(stdout.Bytes(), []byte("Updated successfully")) {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestConsumeUpdateNoticeOnce(t *testing.T) {
	_ = ConsumeUpdateNotice()
	setUpdateNotice("  new version  ")

	if got := ConsumeUpdateNotice(); got != "new version" {
		t.Fatalf("ConsumeUpdateNotice() = %q, want %q", got, "new version")
	}
	if got := ConsumeUpdateNotice(); got != "" {
		t.Fatalf("ConsumeUpdateNotice() second call = %q, want empty", got)
	}
}
