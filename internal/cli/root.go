package cli

import (
	"fmt"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/spf13/cobra"
)

type CLI struct {
	Config  *config.Config
	RootCmd *cobra.Command
}

type CommandDeps struct {
	Config   *config.Config
	Registry *bootloader.Registry
}

func (d *CommandDeps) Bootloader() (bootloader.Bootloader, error) {
	return ResolveBootloader(d.Config.Bootloader.Name, d.Registry)
}

func NewCLI() *CLI {
	cli := &CLI{}

	deps := &CommandDeps{
		Config:   &config.Config{},
		Registry: bootloader.NewRegistry(),
	}

	var cfgFile string

	rootCmd := &cobra.Command{
		Use:   "remote-boot-agent",
		Short: "remote-boot-agent reads boot configurations and posts them to Home Assistant",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Don't load the config if we're also trying to generate it
			if cmd.Name() == "generate-config" {
				return nil
			}

			cfg, err := config.Load(cfgFile, cmd.Flags())
			if err != nil {
				return err
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			*deps.Config = *cfg
			cli.Config = deps.Config
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().String("mac", "", "MAC Address override")
	rootCmd.PersistentFlags().String("hostname", "", "Hostname override")
	rootCmd.PersistentFlags().String("bootloader", "", "Bootloader type override (e.g., grub)")
	rootCmd.PersistentFlags().String("bootloader-path", "", "Bootloader config path override")
	rootCmd.PersistentFlags().String("hass-url", "", "Home Assistant URL override")
	rootCmd.PersistentFlags().String("hass-webhook", "", "Home Assistant Webhook ID override")

	deps.Registry.Register("grub", bootloader.NewGrub)

	rootCmd.AddCommand(NewListCmd(deps))
	rootCmd.AddCommand(NewPushCmd(deps))
	rootCmd.AddCommand(NewGenerateConfigCmd())

	// get rid of the completion command because it doesn't make sense here
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	cli.RootCmd = rootCmd
	return cli
}

func (cli *CLI) Execute() error {
	return cli.RootCmd.Execute()
}

func ResolveBootloader(name string, registry *bootloader.Registry) (bootloader.Bootloader, error) {
	if name != "" {
		bl := registry.Get(name)
		if bl == nil {
			return nil, fmt.Errorf("specified bootloader %s not supported", name)
		}
		return bl, nil
	}

	bl, err := registry.Detect()
	if err != nil {
		return nil, fmt.Errorf("bootloader detection failed: %w", err)
	}
	return bl, nil
}
