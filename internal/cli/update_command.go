package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"neo-code/internal/updater"
	"neo-code/internal/version"
)

type updateCommandOptions struct {
	IncludePrerelease bool
}

var runUpdateCommand = defaultUpdateCommandRunner

// newUpdateCommand 创建 update 子命令并绑定升级参数。
func newUpdateCommand() *cobra.Command {
	options := &updateCommandOptions{}

	cmd := &cobra.Command{
		Use:          "update",
		Short:        "Update neocode to the latest release",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runUpdateCommand(cmd.Context(), *options)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if !result.Updated {
				latest := strings.TrimSpace(result.LatestVersion)
				if latest == "" {
					latest = "unknown"
				}
				_, _ = fmt.Fprintf(out, "Already up-to-date (latest: %s).\n", latest)
				return nil
			}

			_, _ = fmt.Fprintf(out, "Updated successfully: %s -> %s\n", result.CurrentVersion, result.LatestVersion)
			return nil
		},
	}

	cmd.Flags().BoolVar(&options.IncludePrerelease, "prerelease", false, "include prerelease versions")
	return cmd
}

// defaultUpdateCommandRunner 执行手动升级流程并返回升级结果。
func defaultUpdateCommandRunner(ctx context.Context, options updateCommandOptions) (updater.UpdateResult, error) {
	return updater.DoUpdate(ctx, updater.UpdateOptions{
		CurrentVersion:    version.Current(),
		IncludePrerelease: options.IncludePrerelease,
	})
}
