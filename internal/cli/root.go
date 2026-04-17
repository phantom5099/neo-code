package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"neo-code/internal/app"
	"neo-code/internal/config"
	"neo-code/internal/updater"
	"neo-code/internal/version"
)

var launchRootProgram = defaultRootProgramLauncher
var newRootProgram = app.NewProgram
var runGlobalPreload = defaultGlobalPreload
var runSilentUpdateCheck = defaultSilentUpdateCheck

const silentUpdateCheckTimeout = 3 * time.Second

// GlobalFlags 描述 CLI 根命令当前支持的全局参数。
type GlobalFlags struct {
	Workdir string
}

// Execute 负责执行 NeoCode 的 CLI 根命令。
func Execute(ctx context.Context) error {
	app.EnsureConsoleUTF8()
	_ = ConsumeUpdateNotice()
	return NewRootCommand().ExecuteContext(ctx)
}

// NewRootCommand 创建 NeoCode 的 CLI 根命令。
func NewRootCommand() *cobra.Command {
	settings := viper.New()
	flags := &GlobalFlags{}

	cmd := &cobra.Command{
		Use:          "neocode",
		Short:        "NeoCode coding agent",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if shouldSkipGlobalPreload(cmd) {
				return nil
			}
			if err := runGlobalPreload(cmd.Context()); err != nil {
				return err
			}
			runSilentUpdateCheck(cmd.Context())
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.Workdir = strings.TrimSpace(settings.GetString("workdir"))
			return launchRootProgram(cmd.Context(), app.BootstrapOptions{
				Workdir: flags.Workdir,
			})
		},
	}

	cmd.PersistentFlags().String("workdir", "", "工作目录（覆盖本次运行工作区）")
	_ = settings.BindPFlag("workdir", cmd.PersistentFlags().Lookup("workdir"))
	cmd.AddCommand(
		newGatewayCommand(),
		newURLDispatchCommand(),
		newUpdateCommand(),
	)

	return cmd
}

// defaultRootProgramLauncher 负责在默认根命令路径下启动 TUI。
func defaultRootProgramLauncher(ctx context.Context, opts app.BootstrapOptions) (err error) {
	program, cleanup, err := newRootProgram(ctx, opts)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer func() {
			cleanupErr := cleanup()
			if cleanupErr == nil {
				return
			}
			if err == nil {
				err = cleanupErr
				return
			}
			err = errors.Join(err, cleanupErr)
		}()
	}
	_, err = program.Run()
	return err
}

// defaultGlobalPreload runs lightweight startup preloads such as persisted env.
func defaultGlobalPreload(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return config.LoadPersistedEnv("")
}

// defaultSilentUpdateCheck 在后台异步检查新版本并缓存退出后提示文案。
func defaultSilentUpdateCheck(ctx context.Context) {
	currentVersion := version.Current()
	if !version.IsSemverRelease(currentVersion) {
		return
	}
	parentCtx := context.WithoutCancel(ctx)

	go func(parent context.Context, currentVersion string) {
		checkCtx, cancel := context.WithTimeout(parent, silentUpdateCheckTimeout)
		defer cancel()

		result, err := updater.CheckLatest(checkCtx, updater.CheckOptions{
			CurrentVersion:    currentVersion,
			IncludePrerelease: false,
		})
		if err != nil || !result.HasUpdate {
			return
		}
		if strings.TrimSpace(result.LatestVersion) == "" {
			return
		}
		setUpdateNotice(fmt.Sprintf("🚀 发现新版本: %s，运行 neocode update 即可升级", result.LatestVersion))
	}(parentCtx, currentVersion)
}

// shouldSkipGlobalPreload 判断当前命令是否应跳过全局预加载逻辑。
func shouldSkipGlobalPreload(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cmd.Name()), "url-dispatch")
}
