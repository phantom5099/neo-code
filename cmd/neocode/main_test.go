package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neo-code/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeProgram struct {
	runErr error
	called bool
}

func (p *fakeProgram) Run() (tea.Model, error) {
	p.called = true
	return nil, p.runErr
}

func TestLoadDotEnvSetsMissingVarsOnly(t *testing.T) {
	t.Setenv("EXISTING_KEY", "keep-me")
	t.Setenv("NEW_KEY", "")

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "EXISTING_KEY=override\nNEW_KEY= new-value \n# comment\nINVALID_LINE\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := os.Getenv("EXISTING_KEY"); got != "keep-me" {
		t.Fatalf("expected existing env var to be preserved, got %q", got)
	}
	if got := os.Getenv("NEW_KEY"); got != "new-value" {
		t.Fatalf("expected new env var to load, got %q", got)
	}
}

func TestLoadDotEnvIgnoresMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.env")
	if err := loadDotEnv(missing); err != nil {
		t.Fatalf("expected missing file to be ignored, got %v", err)
	}
}

func TestLoadDotEnvReturnsNonENOENTError(t *testing.T) {
	if err := loadDotEnv(t.TempDir()); err == nil {
		t.Fatal("expected non-ENOENT read error")
	}
}

func TestLoadDotEnvTrimsQuotedValuesAndSkipsEmptyKeys(t *testing.T) {
	t.Setenv("QUOTED_KEY", "")

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "QUOTED_KEY=' spaced value '\n =ignored\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := os.Getenv("QUOTED_KEY"); got != " spaced value " {
		t.Fatalf("expected quoted value to be trimmed, got %q", got)
	}
}

func TestParseWorkspaceFlagParsesWorkspaceValue(t *testing.T) {
	stderr := &bytes.Buffer{}

	got, err := parseWorkspaceFlag([]string{"-workspace", "D:/neo-code/workspace"}, stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "D:/neo-code/workspace" {
		t.Fatalf("unexpected workspace flag value %q", got)
	}
}

func TestDefaultRunDepsWiresStandardStreamsAndFunctions(t *testing.T) {
	deps := defaultRunDeps(strings.NewReader("in"), &bytes.Buffer{}, &bytes.Buffer{})

	if deps.stdin == nil || deps.stdout == nil || deps.stderr == nil {
		t.Fatal("expected stdio to be preserved in deps")
	}
	if deps.setUTF8Mode == nil || deps.prepareWorkspace == nil || deps.ensureAPIKeyInteractive == nil || deps.loadAppConfig == nil || deps.newProgram == nil {
		t.Fatal("expected default dependencies to be populated")
	}
}

func TestRunUsesInjectableDepBuilder(t *testing.T) {
	origBuildRunDeps := buildRunDeps
	t.Cleanup(func() { buildRunDeps = origBuildRunDeps })

	called := false
	buildRunDeps = func(stdin io.Reader, stdout, stderr io.Writer) runDeps {
		called = true
		return runDeps{
			stdin:            stdin,
			stdout:           stdout,
			stderr:           stderr,
			setUTF8Mode:      func() {},
			prepareWorkspace: func(string) (string, error) { return "D:/neo-code", nil },
			ensureAPIKeyInteractive: func(context.Context, *bufio.Scanner, string) (bool, error) {
				return false, nil
			},
		}
	}

	err := run("D:/neo-code", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected run to use buildRunDeps")
	}
}

func TestRunWithDepsReturnsWorkspacePreparationError(t *testing.T) {
	stderr := &bytes.Buffer{}

	err := runWithDeps("", runDeps{
		stdin:            strings.NewReader(""),
		stdout:           &bytes.Buffer{},
		stderr:           stderr,
		setUTF8Mode:      func() {},
		prepareWorkspace: func(string) (string, error) { return "", errors.New("workspace failed") },
	})
	if err == nil || !strings.Contains(err.Error(), "workspace failed") {
		t.Fatalf("expected workspace error, got %v", err)
	}
}

func TestRunWithDepsStopsCleanlyWhenSetupNotReady(t *testing.T) {
	stdout := &bytes.Buffer{}
	loadCalled := false

	err := runWithDeps("D:/neo-code", runDeps{
		stdin:            strings.NewReader(""),
		stdout:           stdout,
		stderr:           &bytes.Buffer{},
		setUTF8Mode:      func() {},
		prepareWorkspace: func(string) (string, error) { return "D:/neo-code", nil },
		ensureAPIKeyInteractive: func(_ context.Context, _ *bufio.Scanner, path string) (bool, error) {
			if path != defaultConfigPath {
				t.Fatalf("expected config path %q, got %q", defaultConfigPath, path)
			}
			return false, nil
		},
		loadAppConfig: func(string) error {
			loadCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if loadCalled {
		t.Fatal("loadAppConfig should not run when setup is not ready")
	}
	if !strings.Contains(stdout.String(), "NeoCode") {
		t.Fatalf("expected exit message in stdout, got %q", stdout.String())
	}
}

func TestRunWithDepsReturnsBootstrapError(t *testing.T) {
	err := runWithDeps("D:/neo-code", runDeps{
		stdin:       strings.NewReader(""),
		stdout:      &bytes.Buffer{},
		stderr:      &bytes.Buffer{},
		setUTF8Mode: func() {},
		prepareWorkspace: func(string) (string, error) {
			return "D:/neo-code", nil
		},
		ensureAPIKeyInteractive: func(context.Context, *bufio.Scanner, string) (bool, error) {
			return false, errors.New("bootstrap failed")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "bootstrap failed") {
		t.Fatalf("expected bootstrap error, got %v", err)
	}
}

func TestRunWithDepsReturnsLoadAppConfigError(t *testing.T) {
	err := runWithDeps("D:/neo-code", runDeps{
		stdin:       strings.NewReader(""),
		stdout:      &bytes.Buffer{},
		stderr:      &bytes.Buffer{},
		setUTF8Mode: func() {},
		prepareWorkspace: func(string) (string, error) {
			return "D:/neo-code", nil
		},
		ensureAPIKeyInteractive: func(context.Context, *bufio.Scanner, string) (bool, error) {
			return true, nil
		},
		loadAppConfig: func(string) error { return errors.New("load failed") },
	})
	if err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("expected load error, got %v", err)
	}
}

func TestRunWithDepsPassesHistoryTurnsToProgram(t *testing.T) {
	origGlobalConfig := config.GlobalAppConfig
	t.Cleanup(func() { config.GlobalAppConfig = origGlobalConfig })

	cfg := config.DefaultAppConfig()

	program := &fakeProgram{}
	newProgramCalled := false
	err := runWithDeps("D:/neo-code", runDeps{
		stdin:                   strings.NewReader(""),
		stdout:                  &bytes.Buffer{},
		stderr:                  &bytes.Buffer{},
		setUTF8Mode:             func() {},
		prepareWorkspace:        func(string) (string, error) { return "D:/neo-code", nil },
		ensureAPIKeyInteractive: func(context.Context, *bufio.Scanner, string) (bool, error) { return true, nil },
		loadAppConfig: func(string) error {
			config.GlobalAppConfig = cfg
			return nil
		},
		newProgram: func(historyTurns int, configPath, workspaceRoot string) (programRunner, error) {
			newProgramCalled = true
			if historyTurns != cfg.History.ShortTermTurns {
				t.Fatalf("unexpected history turns %d", historyTurns)
			}
			if configPath != defaultConfigPath || workspaceRoot != "D:/neo-code" {
				t.Fatalf("unexpected program args: %q %q", configPath, workspaceRoot)
			}
			return program, nil
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !newProgramCalled || !program.called {
		t.Fatal("expected program to be created and run")
	}
}

func TestRunWithDepsReturnsNewProgramError(t *testing.T) {
	origGlobalConfig := config.GlobalAppConfig
	t.Cleanup(func() { config.GlobalAppConfig = origGlobalConfig })

	cfg := config.DefaultAppConfig()

	err := runWithDeps("D:/neo-code", runDeps{
		stdin:                   strings.NewReader(""),
		stdout:                  &bytes.Buffer{},
		stderr:                  &bytes.Buffer{},
		setUTF8Mode:             func() {},
		prepareWorkspace:        func(string) (string, error) { return "D:/neo-code", nil },
		ensureAPIKeyInteractive: func(context.Context, *bufio.Scanner, string) (bool, error) { return true, nil },
		loadAppConfig: func(string) error {
			config.GlobalAppConfig = cfg
			return nil
		},
		newProgram: func(int, string, string) (programRunner, error) { return nil, errors.New("new program failed") },
	})
	if err == nil || !strings.Contains(err.Error(), "new program failed") {
		t.Fatalf("expected new program error, got %v", err)
	}
}

func TestRunWithDepsReturnsProgramRunError(t *testing.T) {
	origGlobalConfig := config.GlobalAppConfig
	t.Cleanup(func() { config.GlobalAppConfig = origGlobalConfig })

	cfg := config.DefaultAppConfig()
	program := &fakeProgram{runErr: errors.New("program failed")}

	err := runWithDeps("D:/neo-code", runDeps{
		stdin:                   strings.NewReader(""),
		stdout:                  &bytes.Buffer{},
		stderr:                  &bytes.Buffer{},
		setUTF8Mode:             func() {},
		prepareWorkspace:        func(string) (string, error) { return "D:/neo-code", nil },
		ensureAPIKeyInteractive: func(context.Context, *bufio.Scanner, string) (bool, error) { return true, nil },
		loadAppConfig: func(string) error {
			config.GlobalAppConfig = cfg
			return nil
		},
		newProgram: func(int, string, string) (programRunner, error) { return program, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "program failed") {
		t.Fatalf("expected run error, got %v", err)
	}
}

func TestRunWithDepsHappyPathCallsUTF8Hook(t *testing.T) {
	origGlobalConfig := config.GlobalAppConfig
	t.Cleanup(func() { config.GlobalAppConfig = origGlobalConfig })

	cfg := config.DefaultAppConfig()
	utf8Called := false
	program := &fakeProgram{}

	err := runWithDeps("D:/neo-code", runDeps{
		stdin:  strings.NewReader(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		setUTF8Mode: func() {
			utf8Called = true
		},
		prepareWorkspace:        func(string) (string, error) { return "D:/neo-code", nil },
		ensureAPIKeyInteractive: func(context.Context, *bufio.Scanner, string) (bool, error) { return true, nil },
		loadAppConfig: func(string) error {
			config.GlobalAppConfig = cfg
			return nil
		},
		newProgram: func(int, string, string) (programRunner, error) { return program, nil },
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !utf8Called {
		t.Fatal("expected UTF8 hook to be called")
	}
	if !program.called {
		t.Fatal("expected program to run")
	}
}
