package bootstrap

import (
	"bufio"
	"context"

	runtimebootstrap "neo-code/internal/agentruntime/bootstrap"
)

func PrepareWorkspace(workspaceFlag string) (string, error) {
	return runtimebootstrap.PrepareWorkspace(workspaceFlag)
}

func EnsureAPIKeyInteractive(ctx context.Context, scanner *bufio.Scanner, configPath string) (bool, error) {
	return runtimebootstrap.EnsureAPIKeyInteractive(ctx, scanner, configPath)
}
