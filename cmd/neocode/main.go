package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	runtimebootstrap "neo-code/internal/agentruntime/bootstrap"
	"neo-code/internal/config"
	tuibootstrap "neo-code/internal/tui/bootstrap"

	tea "github.com/charmbracelet/bubbletea"
)

var defaultConfigPath = config.DefaultConfigPath()

var buildRunDeps = defaultRunDeps

type programRunner interface {
	Run() (tea.Model, error)
}

type runDeps struct {
	stdin                   io.Reader
	stdout                  io.Writer
	stderr                  io.Writer
	setUTF8Mode             func()
	prepareWorkspace        func(string) (string, error)
	ensureAPIKeyInteractive func(context.Context, *bufio.Scanner, string) (bool, error)
	loadAppConfig           func(string) error
	newProgram              func(string, string) (programRunner, error)
}

func defaultRunDeps(stdin io.Reader, stdout, stderr io.Writer) runDeps {
	return runDeps{
		stdin:                   stdin,
		stdout:                  stdout,
		stderr:                  stderr,
		setUTF8Mode:             setUTF8Mode,
		prepareWorkspace:        runtimebootstrap.PrepareWorkspace,
		ensureAPIKeyInteractive: runtimebootstrap.EnsureAPIKeyInteractive,
		loadAppConfig:           config.LoadAppConfig,
		newProgram: func(configPath, workspaceRoot string) (programRunner, error) {
			return tuibootstrap.NewProgram(configPath, workspaceRoot)
		},
	}
}

func main() {
	workspaceFlag, err := parseWorkspaceFlag(os.Args[1:], os.Stderr)
	if err != nil {
		os.Exit(1)
	}
	_ = loadDotEnv(".env")

	if err := run(workspaceFlag, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseWorkspaceFlag(args []string, stderr io.Writer) (string, error) {
	fs := flag.NewFlagSet("neocode", flag.ContinueOnError)
	fs.SetOutput(stderr)

	workspaceFlag := fs.String("workspace", "", "override the workspace root")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	return *workspaceFlag, nil
}

func run(workspaceFlag string, stdin io.Reader, stdout, stderr io.Writer) error {
	return runWithDeps(workspaceFlag, buildRunDeps(stdin, stdout, stderr))
}

func runWithDeps(workspaceFlag string, deps runDeps) error {
	if deps.setUTF8Mode != nil {
		deps.setUTF8Mode()
	}

	workspaceRoot, err := deps.prepareWorkspace(workspaceFlag)
	if err != nil {
		return fmt.Errorf("prepare workspace: %w", err)
	}

	scanner := bufio.NewScanner(deps.stdin)
	ready, err := deps.ensureAPIKeyInteractive(context.Background(), scanner, defaultConfigPath)
	if err != nil {
		return fmt.Errorf("bootstrap setup: %w", err)
	}
	if !ready {
		fmt.Fprintln(deps.stdout, "Exiting NeoCode setup.")
		return nil
	}

	if err := deps.loadAppConfig(defaultConfigPath); err != nil {
		return fmt.Errorf("load app config: %w", err)
	}

	p, err := deps.newProgram(defaultConfigPath, workspaceRoot)
	if err != nil {
		return fmt.Errorf("initialize program: %w", err)
	}
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run program: %w", err)
	}

	return nil
}

func loadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		value = strings.Trim(value, `"'`)
		os.Setenv(key, value)
	}

	return nil
}
