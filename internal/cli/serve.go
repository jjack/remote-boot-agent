package cli

import (
	"context"
	"log/slog"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/daemon"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/host"
	"github.com/jjack/grubstation/internal/reporter"
	"github.com/jjack/grubstation/internal/version"
	"github.com/spf13/cobra"
)

type serveRunner interface {
	Run(ctx context.Context) error
}

var newServe = func(cfg daemon.Config, meta daemon.Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) serveRunner {
	return daemon.New(cfg, meta, regHandler, updateHandler)
}

func NewServeCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the persistent agent daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			var regHandler func(ctx context.Context, token string) error
			var updateHandler func(ctx context.Context) error

			mgr, _ := deps.Manager(cmd.Context())
			mgrName := ""
			if activeMgr := mgr; activeMgr != nil {
				mgrName = activeMgr.Name()
			}

			rep := reporter.New(deps.Config, deps.Grub, mgrName)
			regHandler = rep.RegisterDaemon
			updateHandler = rep.PushBootOptions

			if deps.Config.Daemon.ReportBootOptions {
				// Drift detection
				waitTime := config.DefaultGrubWaitSeconds
				targetURL := deps.Config.HomeAssistant.URL
				if deps.Config.Grub != nil {
					if deps.Config.Grub.WaitTimeSeconds != 0 {
						waitTime = deps.Config.Grub.WaitTimeSeconds
					}
					if deps.Config.Grub.URL != "" {
						targetURL = deps.Config.Grub.URL
					}
				}

				drift, err := deps.Grub.CheckDrift(grub.SetupOptions{
					TargetMAC:       deps.Config.Host.MACAddress,
					TargetURL:       targetURL,
					AuthToken:       deps.Config.HomeAssistant.WebhookID,
					WaitTimeSeconds: waitTime,
				})
				if err == nil && drift {
					slog.Warn("GRUB configuration drift detected. Your installed GRUB script does not match the current config. Run 'grubstation setup --apply' to sync.")
				} else if err != nil {
					slog.Debug("Failed to check for GRUB drift", "error", err)
				}
			}
			d := newServe(daemon.Config{
				Port:              deps.Config.Daemon.Port,
				ReportBootOptions: deps.Config.Daemon.ReportBootOptions,
				APIKey:            deps.Config.Daemon.APIKey,
			}, daemon.Metadata{
				OS:             host.Platform(),
				Version:        version.Version,
				ServiceManager: mgrName,
			}, regHandler, updateHandler)
			return d.Run(cmd.Context())
		},
	}
}
