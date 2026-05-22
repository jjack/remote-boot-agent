package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jjack/grubstation/internal/cli/wizard"
	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/host"
	"github.com/jjack/grubstation/internal/reporter"
	"github.com/jjack/grubstation/internal/servicemanager"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yarlson/tap"
)

var osMkdirAll = os.MkdirAll

var ErrElevated = errors.New("elevated")

func performInstall(cmd *cobra.Command, deps *CommandDeps, cfgFile string, token string) error {
	slog.Debug("Starting installation process", "config", cfgFile)
	mgr, err := deps.Manager(cmd.Context())
	if err != nil {
		slog.Debug("Failed to get service manager", "error", err)
		return err
	}
	slog.Debug("Using service manager", "manager", mgr.Name())

	if err := mgr.CheckPermissions(cmd.Context()); err != nil {
		slog.Debug("Permission check failed", "error", err)
		return err
	}

	absConfig, err := filepath.Abs(cfgFile)
	if err != nil {
		slog.Debug("Failed to resolve absolute config path", "path", cfgFile, "error", err)
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	if deps.Config.Daemon.ReportBootOptions {
		waitTime := config.DefaultGrubWaitSeconds
		targetURL := deps.Config.HomeAssistant.URL
		if deps.Config.Grub != nil {
			waitTime = deps.Config.Grub.WaitTimeSeconds
			if deps.Config.Grub.URL != "" {
				targetURL = deps.Config.Grub.URL
			}
		}

		opts := grub.SetupOptions{
			TargetMAC:       deps.Config.Host.MACAddress,
			TargetURL:       targetURL,
			AuthToken:       deps.Config.HomeAssistant.WebhookID,
			WaitTimeSeconds: waitTime,
		}

		warning := deps.Grub.SetupWarning()
		tap.Message("Installing into grub...", tap.MessageOptions{
			Hint: warning,
		})
		slog.Debug("Installing GRUB script", "mac", opts.TargetMAC, "url", opts.TargetURL, "waitTime", opts.WaitTimeSeconds)
		if err := deps.Grub.Setup(cmd.Context(), opts); err != nil {
			slog.Debug("GRUB setup failed", "error", err)
			return fmt.Errorf("failed to install grub: %w", err)
		}

		tap.Message("Pushing initial boot options to Home Assistant...")
		activeMgr, _ := deps.Manager(cmd.Context())
		mgrName := ""
		if activeMgr != nil {
			mgrName = activeMgr.Name()
		}
		rep := reporter.New(deps.Config, deps.Grub, mgrName)

		if token != "" {
			slog.Debug("Registering daemon with token")
			if err := rep.RegisterDaemon(cmd.Context(), token); err != nil {
				slog.Debug("Daemon registration failed", "error", err)
				return err
			}
		}

		slog.Debug("Pushing boot options to HA")
		if err := rep.PushBootOptions(cmd.Context()); err != nil {
			slog.Debug("Failed to push boot options", "error", err)
			return err
		}
		tap.Message("Successfully pushed initial state to Home Assistant.")
	}

	tap.Message(fmt.Sprintf("Installing into service manager: %s", mgr.Name()))
	slog.Debug("Configuring service manager")
	if err := mgr.Configure(cmd.Context(), deps.Config); err != nil {
		slog.Debug("Service manager configuration failed", "error", err)
		return fmt.Errorf("failed to configure service: %w", err)
	}

	slog.Debug("Installing service", "configPath", absConfig)
	if err := mgr.Install(cmd.Context(), absConfig); err != nil {
		slog.Debug("Service installation failed", "error", err)
		return fmt.Errorf("failed to install manager: %w", err)
	}

	tap.Message("Starting service...")
	slog.Debug("Starting service")
	if err := mgr.Start(cmd.Context()); err != nil {
		slog.Debug("Service start failed", "error", err)
		return fmt.Errorf("failed to start service: %v", err)
	}

	slog.Debug("Installation completed successfully")
	tap.Message("Installation completed successfully.")

	return nil
}

func ensureSupport(ctx context.Context, deps *CommandDeps) (servicemanager.Manager, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	mgr, err := deps.Registry.Detect(ctx)
	if err != nil {
		if errors.Is(err, servicemanager.ErrNotSupported) {
			supported := strings.Join(deps.Registry.SupportedServices(), ", ")
			return nil, fmt.Errorf("no supported service manager detected. Please ensure you have one of the following installed: %s", supported)
		}
		return nil, err
	}
	return mgr, nil
}

func IsInstalled(ctx context.Context, deps *CommandDeps) (bool, error) {
	mgr, err := ensureSupport(ctx, deps)
	if err != nil {
		return false, err
	}
	return mgr.IsInstalled(ctx)
}

func NewSetupCmd(deps *CommandDeps) *cobra.Command {
	var applyOnly bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Run the automated setup wizard to configure and install the agent",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if applyOnly {
				// For apply, we WANT the default config loading to happen
				return nil
			}
			return nil // Override root config loading, we are generating it from scratch
		},
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			dump := setupDebugLogging()
			defer func() { dump(err) }()

			if runtime.GOOS == "windows" {
				defer func() {
					if err == ErrElevated {
						return
					}

					if err != nil {
						tap.Outro(fmt.Sprintf("Error: %v", err))
					}

					// Always wait on Windows when running the setup wizard so the user can see the output.
					// We tell the user they can close the window manually because stdin can be unreliable in some MSI-launched environments.
					fmt.Print("\nSetup finished. You can now close this window.")
					s := bufio.NewScanner(os.Stdin)
					s.Scan()
				}()
			}

			if applyOnly {
				cfgPath, _ := cmd.Flags().GetString("config")
				return performInstall(cmd, deps, cfgPath, "")
			}

			mgr, err := ensureSupport(cmd.Context(), deps)
			if err != nil {
				return err
			}

			if !dryRun {
				if err := mgr.CheckPermissions(cmd.Context()); err != nil {
					return err
				}
			}

			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil || cfgPath == "" {
				cfgPath = config.DefaultConfigPath()
			}

			var currentPort int
			if _, err := os.Stat(cfgPath); err == nil {
				v := viper.New()
				v.SetConfigFile(cfgPath)
				if err := v.ReadInConfig(); err == nil {
					currentPort = v.GetInt("daemon.port")
				}
			}

			if !dryRun {
				if err := osMkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
					return fmt.Errorf("failed to create config directory: %w", err)
				}
			}

			// Clear the terminal screen before starting the interactive wizard
			cmd.Print("\033[H\033[2J")

			tap.Intro("GrubStation Setup")

			isConfigured := false
			if _, err := os.Stat(cfgPath); err == nil {
				isConfigured = true
			}

			// Perform initial discovery
			hostname, _ := host.DetectHostname()
			interfaces, _ := host.GetWOLInterfaces()
			grubConfigPath, _ := deps.Grub.DiscoverConfigPath(cmd.Context())

			state := wizard.SystemState{
				Hostname:       hostname,
				Interfaces:     interfaces,
				GrubConfigPath: grubConfigPath,
				IsReinstall:    isConfigured,
				CurrentPort:    currentPort,
			}

			cfg, err := wizard.RunGenerateSurvey(cmd.Context(), state, dryRun)
			if err != nil {
				if errors.Is(err, wizard.ErrAborted) {
					tap.Message("Setup aborted.")
					tap.Outro("Goodbye!")
					return nil
				}
				return err
			}

			if dryRun {
				wizard.PrintConfigSummary(cmd, cfg, cfgPath)

				if svcPreview, err := mgr.Preview(cmd.Context(), cfgPath); err == nil {
					tap.Box(svcPreview, fmt.Sprintf(" %s Service Preview ", mgr.Name()), tap.BoxOptions{
						ContentPadding: 2,
					})
				}

				if cfg.Daemon.ReportBootOptions {
					waitTime := config.DefaultGrubWaitSeconds
					targetURL := cfg.HomeAssistant.URL
					if cfg.Grub != nil {
						waitTime = cfg.Grub.WaitTimeSeconds
						if cfg.Grub.URL != "" {
							targetURL = cfg.Grub.URL
						}
					}
					grubPreview, err := deps.Grub.GenerateScript(grub.SetupOptions{
						TargetMAC:       cfg.Host.MACAddress,
						TargetURL:       targetURL,
						AuthToken:       cfg.HomeAssistant.WebhookID,
						WaitTimeSeconds: waitTime,
					})
					if err == nil {
						tap.Box(grubPreview, " GRUB Script Preview (/etc/grub.d/99_grubstation) ", tap.BoxOptions{
							ContentPadding: 2,
						})
					}
				}

				tap.Message("Dry run completed. Configuration shown above was not saved.")
				tap.Outro("Dry run finished")
				return nil
			}

			if err := config.Save(cfg, cfgPath); err != nil {
				return err
			}

			tap.Outro("Configuration setup complete.", tap.MessageOptions{
				Hint: fmt.Sprintf("saved to: %s", cfgPath),
			})

			tap.Intro("Proceeding with installation...")
			// We update the deps config with our freshly generated config so the installer can use it
			*deps.Config = *cfg
			if err := performInstall(cmd, deps, cfgPath, ""); err != nil {
				if err == ErrElevated {
					return nil
				}
				return err
			}

			tap.Message("Pushing initial boot options to Home Assistant...")
			activeMgr, _ := deps.Manager(cmd.Context())
			mgrName := ""
			if activeMgr != nil {
				mgrName = activeMgr.Name()
			}
			rep := reporter.New(deps.Config, deps.Grub, mgrName)
			if err := rep.PushBootOptions(cmd.Context()); err != nil {
				return err
			}
			tap.Message("Successfully pushed initial state to Home Assistant.")

			tap.Outro("Setup complete!")
			return nil
		},
	}

	cmd.Flags().BoolVar(&applyOnly, "apply", false, "Skip survey and install service based on current config")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview configuration without saving or installing")

	return cmd
}
